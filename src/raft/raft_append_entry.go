package raft

import (
	"sort"
	"time"
)

//
// example RequestVote RPC arguments structure.
// field names must start with capital letters!
//
type AppendEntriesArgs struct {
	Term int
	LeaderId int
	PrevLogIndex int
	PrevLogTerm int
	Entries []Log
	LeaderCommit int
}

//
// example RequestVote RPC reply structure.
// field names must start with capital letters!
//
type AppendEntriesReply struct {
	Term int
	Success bool
	ConflictTerm int
	ConflictIndex int
}

//when appending logs to server idx , get them
func (rf *Raft) getAppendLogs(idx int) []Log {
	if idx==rf.me {
		panic("can't append log to self")
	}
	nid:=rf.getLogIdxByRealIdx(rf.nextIndex[idx])
	return rf.log[nid:]
}


//
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	// Your code here (2A, 2B).
	rf.lock("dealingAppendEntries")
	defer rf.unlock("dealingAppendEntries")

	reply.Success=false
	reply.Term=rf.currentTerm
	reply.ConflictTerm=-1

	argTerm:=args.Term
	//condition 1(in figure 2 appendEntries)
	if argTerm<rf.currentTerm {
		return
	} else if args.Term>rf.currentTerm{
		rf.changeRole(Follower)
	}else if rf.role==Candidate&&args.Term==rf.currentTerm{
		//see raftScope for this rule(I haven't found it in raft paper)
		rf.changeRole(Follower)
	}

	rf.resetElectionTimer()

	//condition2
	prevLogidx:=args.PrevLogIndex
	//prevLogidx对应的目前的idx 如果为0 ，理论上也可以，因为term=lastsnapshot
	realPrevIdx:=rf.getLogIdxByRealIdx(prevLogidx)
	if realPrevIdx>= len(rf.log){
		DPrintf("dealing append entries fail1 server id:%d, log start:%d,log len:%d,success:%v,rf log len:%d,time:%v\n",rf.me,args.PrevLogIndex,len(args.Entries),reply.Success, len(rf.log),time.Now().Sub(rf.startTime))
		reply.ConflictIndex=len(rf.log)+rf.lastSnapshotIdx
		return
	}
	if rf.log[realPrevIdx].Term!=args.PrevLogTerm {
		DPrintf("dealing append entries fail2 server id:%d, log start:%d,log len:%d,success:%v,time:%v\n",rf.me,args.PrevLogIndex,len(args.Entries),reply.Success,time.Now().Sub(rf.startTime))
		//skip a term
		for i:=realPrevIdx;i>=0 ;i--  {
			if rf.log[i].Term!=rf.log[realPrevIdx].Term {
				reply.ConflictIndex=i+1+rf.lastSnapshotIdx
				break
			}
		}
		if reply.ConflictIndex==0 {
			reply.ConflictIndex=1+rf.lastSnapshotIdx
		}
		reply.ConflictTerm=rf.log[realPrevIdx].Term
		return
	}

	reply.Success=true
	reply.ConflictIndex=-1



	//condition3,4
	rf.log=rf.log[:realPrevIdx+1]
	rf.log=append(rf.log,args.Entries...)
	rf.persist()
	//condition5
	leadercommit:=args.LeaderCommit
	theLastEntryIdx:= len(rf.log)-1+rf.lastSnapshotIdx
	if rf.commitIndex<leadercommit {
		if leadercommit<theLastEntryIdx {
			rf.commitIndex=leadercommit
		}else {
			rf.commitIndex=theLastEntryIdx
		}
	}
	DPrintf("dealing append entries server id:%d, log start:%d,log len:%d,leaderCommit:%d,time:%v\n",rf.me,args.PrevLogIndex,len(args.Entries),args.LeaderCommit,time.Now().Sub(rf.startTime))


}

func (rf* Raft) appendEntriesToFollower(idx int)  bool{
	rf.lock("appendEntries")
	//也许是记录中的timer bug所导致，只能验证当前角色，不是leader则丢弃这次操作（很不优雅的实现，也许教授说的对，真的应该抛弃leader）
	if rf.role!=Leader{
		rf.unlock("appendEntries")
		return false
	}

	//nextIndex[idx] < lastSnapshotIdx => use snapshot instead of entries
	if rf.nextIndex[idx]<rf.lastSnapshotIdx {
		go rf.sendSnapshot(idx)
		rf.unlock("appendEntries")
		return true
	}

	toleranceTimer:=time.NewTimer(RpcToleranceTimeOut*time.Millisecond)
	defer toleranceTimer.Stop()
	args:=&AppendEntriesArgs{}
	reply:=&AppendEntriesReply{}
	//init args
	args.LeaderCommit=rf.commitIndex
	args.LeaderId=rf.me
	args.Term=rf.currentTerm
	args.Entries=rf.getAppendLogs(idx)
	args.PrevLogIndex=rf.nextIndex[idx]-1
	prevLogidx2Realidx:=rf.getLogIdxByRealIdx(args.PrevLogIndex)
	args.PrevLogTerm=rf.log[prevLogidx2Realidx].Term

	DPrintf("leader append entries server id:%d,role:%d, term:%d ,target idx:%d,log start:%d,log length:%d,time:%v\n",rf.me,rf.role,rf.currentTerm,idx,args.PrevLogIndex,len(args.Entries),time.Now().Sub(rf.startTime))
	rf.appendEntriesTimers[idx].Reset(HeartBeatTimeOut*time.Millisecond)
	rf.unlock("appendEntries")

	boolchan:=make(chan bool)

	go func() {

		//与选举同样，这里改为异步，否则服务器将无法响应rpc?(明明没有加锁，为什么?)
		ok:=rf.peers[idx].Call("Raft.AppendEntries", args, reply)
		boolchan<-ok

	}()




	for   {
		select {
			case <-rf.stopSignal:
				return false
			case <-toleranceTimer.C:
				return false;
			case ok:=<-boolchan:{
				rf.lock("appendEntries_2")
				defer rf.unlock("appendEntries_2")
				if rf.role!=Leader {
					return ok
				}
				if ok {
					term:=reply.Term
					success:=reply.Success
					if !success {
						if term>rf.currentTerm {
							DPrintf("term less than target,turn to follower\n")
							rf.currentTerm=term
							rf.persist()
							rf.changeRole(Follower)
						}else {
							//target server refuse logs, reduce log idx
							DPrintf("target server refuse logs,target id:%d,reply term:%d,\n",idx,reply.Term)
							conTerm:=reply.ConflictTerm
							if conTerm==-1 {
								rf.nextIndex[idx]=reply.ConflictIndex
							}else {
								find:=false
								for i:=args.PrevLogIndex-rf.lastSnapshotIdx;i>=0 ;i--  {
									if rf.log[i].Term==conTerm {
										rf.nextIndex[idx]=i+1+rf.lastSnapshotIdx
										find=true
										break
									}
								}
								if !find {
									rf.nextIndex[idx]=reply.ConflictIndex
								}
							}
							rf.appendEntriesTimers[idx].Reset(0)
						}
					} else {
						nextidx:=rf.nextIndex[idx]+ len(args.Entries)
						rf.nextIndex[idx]=nextidx
						rf.matchedIndex[idx]=nextidx-1
						rf.updateCommitIndex()
					}
				}else {
					DPrintf("append entries network failed, server id :%d, time: %v",rf.me, time.Now().Sub(rf.startTime))
					rf.appendEntriesTimers[idx].Reset(10*time.Millisecond)
				}
				return ok
			}
		}
	}
}

func (rf *Raft) updateCommitIndex() {
	if rf.role!=Leader {
		DPrintf("fatal: update commitIndex but not a leader server id:%d",rf.me)
		panic("update commitIndex but not a leader")
	}

	//find median => commitIndex
	idx,_:=rf.getLastLogIdxAndTerm()
	rf.matchedIndex[rf.me]=idx
	temp:=make([]int, len(rf.peers))
	copy(temp,rf.matchedIndex)

	sort.Ints(temp)
	//4:1;5:2
	//matchedindex中位数，如果term=本期term 则可以提交，
	// 如果term小于本期term则不更新（
	// 因为考虑到term一定递增，前面也不可能有符合的出现）
	median:=temp[(len(rf.peers)-1)/2]
	if median>rf.commitIndex&& rf.log[median].Term==rf.currentTerm{
		rf.commitIndex=median
	}
}
