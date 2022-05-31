package raft

import (
	"log"
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
	entries []Log
	leaderCommit int
}

//
// example RequestVote RPC reply structure.
// field names must start with capital letters!
//
type AppendEntriesReply struct {
	term int
	success bool
}

//
// example RequestVote RPC handler.
//
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	// Your code here (2A, 2B).

	rf.lock("dealingAppendEntries")
	defer rf.unlock("dealingAppendEntries")

	log.Printf("dealing append entries server id:%d, server role:%d,arg term:%d,time:%v\n",rf.me,rf.role,args.Term,time.Now().Sub(rf.startTime))


	if args.Term>rf.currentTerm{
		rf.changeRole(Follower)
		return
	}
	if rf.role==Follower {
		rf.electionTimer.Reset(ElectionTimeout*time.Millisecond)
	}
	if rf.role==Candidate&&args.Term>=rf.currentTerm{
		rf.changeRole(Follower)
	}
	
}

func (rf* Raft) appendEntriesToFollower(idx int)  bool{
	rf.lock("appendEntries")
	toleranceTimer:=time.NewTimer(RpcToleranceTimeOut*time.Millisecond)
	defer toleranceTimer.Stop()
	args:=&AppendEntriesArgs{}
	reply:=&AppendEntriesReply{}
	args.Term=rf.currentTerm
	log.Printf("leader append entries server id:%d, server role:%d, term:%d ,target idx:%d,time:%v\n",rf.me,rf.role,rf.currentTerm,idx,time.Now().Sub(rf.startTime))
	rf.appendEntriesTimers[idx].Reset(HeartBeatTimeOut*time.Millisecond)
	rf.unlock("appendEntries")
	boolchan:=make(chan bool)

	go func() {

		//同樣的理由，這裡改為異步，如果卡在這裡，服务器将无法响应rpc?(明明没有加锁，为什么?)
		ok:=rf.peers[idx].Call("Raft.AppendEntries", args, reply)
		boolchan<-ok

	}()

	for   {
		select {
			case <-rf.stopSignal:
				return false
			case <-toleranceTimer.C:
				return false;
			case ok:=<-boolchan:
				return ok
		}
	}
}