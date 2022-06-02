package raft

import (
	"log"
	"sync"
	"time"
)

//
// example RequestVote RPC arguments structure.
// field names must start with capital letters!
//
type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	Term int
	CandidateId int
	LastLogIndex int
	LastLogTerm int
}

//
// example RequestVote RPC reply structure.
// field names must start with capital letters!
//
type RequestVoteReply struct {
	// Your data here (2A).
	Term int
	VoteGranted bool
}

//
// example RequestVote RPC handler.
//
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	rf.lock("dealingRequestVote")
	defer rf.unlock("dealingRequestVote")
	reply.Term=rf.currentTerm
	reply.VoteGranted=false
	
	reqTerm:=args.Term
	reqLogTerm:=args.LastLogTerm
	reqLogIdx:=args.LastLogIndex
	
	if rf.currentTerm>reqTerm{
		return
	}else {
		idx,term:=rf.getLastLogIdxAndTerm()
		//如果任期比请求小，需要重置votefor
		if rf.currentTerm<reqTerm {
			rf.currentTerm=reqTerm
			rf.voteFor=-1
			rf.changeRole(Follower)
		}

		if rf.voteFor==args.CandidateId {
			rf.resetElectionTimer()
			reply.VoteGranted=true
			return
		}else if rf.voteFor!=-1&&rf.voteFor!=args.CandidateId{
			return
		}else {
			if reqLogTerm<term||(reqLogTerm==term&&reqLogIdx<idx) {
				return
			}else {
				rf.voteFor=args.CandidateId
				reply.VoteGranted=true
				rf.resetElectionTimer()
				return
			}
		}
	}
}


func (rf *Raft) resetElectionTimer() {
	rf.electionTimer.Stop();
	rf.electionTimer.Reset(rf.generateRandomElectionTimeOut())
}


func (rf *Raft) startElection() {
	//解决死锁：减小粒度
	rf.lock("startElection")
	rf.changeRole(Candidate)
	log.Printf("start election, id:%d, term:%d, time:%v",rf.me,rf.currentTerm,time.Now().Sub(rf.startTime))

	//是否需要defer?
	//defer rf.resetElectionTimer()

	wg:=sync.WaitGroup{}
	reqArg:=&RequestVoteArgs{}
	reqArg.Term=rf.currentTerm
	reqArg.CandidateId=rf.me

	reqArg.LastLogIndex,reqArg.LastLogTerm=rf.getLastLogIdxAndTerm()

	rf.unlock("startElection")

	//vote for self
	voteGathered:=1
	//gather voteReplys
	replys:=make([]*RequestVoteReply,0)
	for i:=0;i< len(rf.peers);i++  {
		idx:=i
		reqReply:=&RequestVoteReply{}
		reqReply.VoteGranted=false
		if idx==rf.me {
			continue
		}
		wg.Add(1)
		go func() {
			flag:=rf.sendRequestVote(idx,reqArg,reqReply)
			//fmt.Printf("self-id:%d,target-id:%d,flag:%t\n",rf.me,idx,flag)
			//network failure
			if !flag {
				//log.Printf("not receive vote result, server id:%d,target id:%d,my term:%d,reply term:%d,vote granted:%v\n",rf.me,idx,rf.currentTerm,reqReply.Term,reqReply.VoteGranted)
				replys=append(replys, reqReply)
			}else {
				//log.Printf("received vote result, server id:%d,target id:%d,my term:%d,reply term:%d,vote granted:%v\n",rf.me,idx,rf.currentTerm,reqReply.Term,reqReply.VoteGranted)
				replys=append(replys, reqReply)
			}
			wg.Done()
		}()
	}
	wg.Wait()

	//如果直接进入这段代码，存在隐患，有锁-无锁-有锁 。
	//问题是，如果无锁的一段中raft 收到了term更高的 信息，从而变成了Follower怎么办？
	//所以判断第三段中的身份，如果变换则废弃处理
	rf.lock("startElection_dealing_result")
	defer rf.unlock("startElection_dealing_result")

	if rf.role!=Candidate {
		log.Printf("role changing while election ,id :%d",rf.me)
		return
	}

	for _,reply:=range(replys)  {
		if reply.VoteGranted{
			voteGathered++
		}else {
			if reply.Term>rf.currentTerm {
				rf.changeRole(Follower)
				//log.Printf("lose election, id:%d, term:%d, time:%v",rf.me,rf.currentTerm,time.Now().Sub(rf.startTime))
				return
			}
		}
	}

	if voteGathered>=len(rf.peers)/2+1 {
		//todo: become leader
		log.Printf("become leader, id:%d, term:%d, time:%v",rf.me,rf.currentTerm,time.Now().Sub(rf.startTime))
		rf.changeRole(Leader)
	}else {
		//log.Printf("lose election, id:%d, term:%d, time:%v",rf.me,rf.currentTerm,time.Now().Sub(rf.startTime))
		rf.resetElectionTimer()
	}
	//log.Printf("lose election, id:%d, term:%d, time:%v",rf.me,rf.currentTerm,time.Now().Sub(rf.startTime))

}

//
//
// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
//The labrpc package simulates a lossy network, in which servers
//may be unreachable, and in which requests and replies may be lost.
//Call() sends a request and waits for a reply. If a reply arrives
//within a timeout interval, Call() returns true; otherwise
//Call() returns false. Thus Call() may not return for a while.
//A false return can be caused by a dead server, a live server that
//can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
//
//无锁方法，但是不涉及rf字段
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool{
	toleranceTimer:=time.NewTimer(RpcToleranceTimeOut*time.Millisecond)
	//now:=time.Now()
	defer toleranceTimer.Stop()
	boolchan:=make(chan bool)
	go func() {
		for  {
			select {
			case <-rf.stopSignal:{
				boolchan<-false
			}
			case <-toleranceTimer.C:{
				//fmt.Printf("tolerance time:%v\n",time.Now().Sub(now))
				boolchan<-false
			}
			default:
				{
					//当网络出错时，call可能会花费非常多的时间返回，我们这里希望在100ms之内返回，所以这里使用异步
					ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
					//if !ok {
					//	time.Sleep(10 * time.Millisecond)
					//	fmt.Printf("fail time:%v\n",time.Now().Sub(now))
					//	continue
					//} else {
					//	fmt.Printf("true time:%v\n",time.Now().Sub(now))
					//	boolchan<-true
					//}
					boolchan<-ok
				}
			}
		}
	}()

	for   {
		select {
			case ok:=<-boolchan:{
				if !ok{
					continue
				}else {
					return true
				}
			}
			case <-rf.stopSignal:
				return false
			case <-toleranceTimer.C:
				return false

		}
	}

}
