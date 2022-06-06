package raft

import (
	"log"
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
	NextIndex int
}

//when appending logs to server idx , get them
func (rf *Raft) getAppendLogs(idx int) []Log {
	if idx==rf.me {
		panic("can't append log to self")
	}
	nid:=rf.nextIndex[idx]
	return rf.log[nid:]
}


//
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	// Your code here (2A, 2B).

	rf.lock("dealingAppendEntries")
	defer rf.unlock("dealingAppendEntries")


	reply.Success=false
	reply.Term=rf.currentTerm

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

	//condition2
	prevLogidx:=args.PrevLogIndex
	if prevLogidx>= len(rf.log) {
		log.Printf("dealing append entries fail1 server id:%d, log start:%d,log len:%d,success:%v,rf log len:%d,time:%v\n",rf.me,args.PrevLogIndex,len(args.Entries),reply.Success, len(rf.log),time.Now().Sub(rf.startTime))
		reply.NextIndex=len(rf.log)
		//reply.NextIndex=prevLogidx
		return
	}
	if prevLogidx!=0&&rf.log[prevLogidx].Term!=args.PrevLogTerm {
		log.Printf("dealing append entries fail2 server id:%d, log start:%d,log len:%d,success:%v,time:%v\n",rf.me,args.PrevLogIndex,len(args.Entries),reply.Success,time.Now().Sub(rf.startTime))
		if rf.log[prevLogidx].Term>args.PrevLogIndex {
			//skip a term
			for i:=prevLogidx;i>=0 ;i--  {
				if rf.log[i].Term<rf.log[prevLogidx].Term {
					reply.NextIndex=i+1
					break
				}
			}
		}else {
			reply.NextIndex=prevLogidx
		}
		reply.NextIndex=prevLogidx
		return
	}

	reply.Success=true
	reply.NextIndex=-1

	if rf.role==Follower {
		rf.electionTimer.Reset(ElectionTimeout*time.Millisecond)
	}

	//condition3,4
	rf.log=rf.log[:prevLogidx+1]
	rf.log=append(rf.log,args.Entries...)
	rf.persist()
	//condition5
	leadercommit:=args.LeaderCommit
	theLastEntryIdx:= len(rf.log)-1
	if rf.commitIndex<leadercommit {
		if leadercommit<theLastEntryIdx {
			rf.commitIndex=leadercommit
		}else {
			rf.commitIndex=theLastEntryIdx
		}
	}
	log.Printf("dealing append entries server id:%d, log start:%d,log len:%d,leaderCommit:%d,time:%v\n",rf.me,args.PrevLogIndex,len(args.Entries),args.LeaderCommit,time.Now().Sub(rf.startTime))


}

func (rf* Raft) appendEntriesToFollower(idx int)  bool{
	rf.lock("appendEntries")
	//也许是记录中的timer bug所导致，只能验证当前角色，不是leader则丢弃这次操作（很不优雅的实现，也许教授说的对，真的应该抛弃leader）
	if rf.role!=Leader{
		rf.unlock("appendEntries")
		return false
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
	if args.PrevLogIndex>=0 {
		args.PrevLogTerm=rf.log[args.PrevLogIndex].Term
	}else {
		args.PrevLogTerm=0
	}

	log.Printf("leader append entries server id:%d,role:%d, term:%d ,target idx:%d,log start:%d,log length:%d,time:%v\n",rf.me,rf.role,rf.currentTerm,idx,args.PrevLogIndex,len(args.Entries),time.Now().Sub(rf.startTime))
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
						log.Printf("target id and term: %d, %d\n",idx,term)
						if term>rf.currentTerm {
							log.Printf("term less than target,turn to follower\n")
							rf.currentTerm=term
							rf.persist()
							rf.changeRole(Follower)
							//return ok
						}else {
							//target server refuse logs, reduce log idx
							log.Printf("target server refuse logs,target id:%d,reply term:%d,\n",idx,reply.Term)
							//rf.nextIndex[idx]--;
							rf.nextIndex[idx]=reply.NextIndex
							rf.appendEntriesTimers[idx].Reset(0)
						}
					}else {
						nextidx:=rf.nextIndex[idx]+ len(args.Entries)
						rf.nextIndex[idx]=nextidx
						rf.matchedIndex[idx]=nextidx-1
						rf.updateCommitIndex()
					}

				}
				return ok
			}
		}
	}
}

func (rf *Raft) updateCommitIndex() {
	if rf.role!=Leader {
		log.Printf("fatal: update commitIndex but not a leader server id:%d",rf.me)
		panic("update commitIndex but not a leader")
	}

	//find median => commitIndex
	idx,_:=rf.getLastLogIdxAndTerm()
	rf.matchedIndex[rf.me]=idx
	temp:=make([]int, len(rf.peers))
	copy(temp,rf.matchedIndex)

	sort.Ints(temp)
	//4:1;5:2
	median:=temp[(len(rf.peers)-1)/2]
	if median>rf.commitIndex {
		rf.commitIndex=median
	}
}
