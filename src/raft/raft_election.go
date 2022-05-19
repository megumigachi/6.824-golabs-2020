package raft

import (
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
}


func (rf *Raft) resetElectionTimer() {
	rf.electionTimer.Stop();
	rf.electionTimer.Reset(rf.generateRandomElectionTimeOut())
}


func (rf *Raft) startElection() {
	rf.lock("startElection")
	defer rf.unlock("startElection")
	//是否需要defer?
	//defer rf.resetElectionTimer()

	wg:=sync.WaitGroup{}
	reqArg:=&RequestVoteArgs{}
	reqArg.Term=rf.currentTerm
	reqArg.CandidateId=rf.me
	reqArg.LastLogIndex,reqArg.LastLogTerm=rf.getLastLogIdxAndTerm()


	//vote for self
	voteGathered:=1
	//gather voteReplys
	replys:=make([]*RequestVoteReply,0)
	for i:=0;i< len(rf.peers);i++  {
		reqReply:=&RequestVoteReply{}
		if i==rf.me {
			continue
		}
		wg.Add(1)
		go func() {
			flag:=rf.sendRequestVote(i,reqArg,reqReply)
			//network failure
			if !flag {
			}else {
				replys=append(replys, reqReply)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	for _,reply:=range(replys)  {
		if reply.VoteGranted{
			voteGathered++
		}else {
			if reply.Term>rf.currentTerm {
				rf.changeRole(Follower)
				return
			}
		}
	}

	if voteGathered>=len(rf.peers)/2+1 {
		//todo: become leader
		rf.changeRole(Leader)
	}

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
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool{
	toleranceTimer:=time.NewTimer(RpcToleranceTimeOut)
	defer toleranceTimer.Stop()
	for  {
		select {
			case <-toleranceTimer.C:
				return false
			default:
			{
				//Call always return, so it will end when tolerance timer goes out
				//or ok is true
				ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
				if !ok {
					time.Sleep(10 * time.Millisecond)
					continue
				} else {
					return true
				}
			}
		}
	}


}
