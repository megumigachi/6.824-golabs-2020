package raft

import (
	"fmt"
	"time"
)

//
// example RequestVote RPC arguments structure.
// field names must start with capital letters!
//
type AppendEntriesArgs struct {
	term int
}

//
// example RequestVote RPC reply structure.
// field names must start with capital letters!
//
type AppendEntriesReply struct {

}

//
// example RequestVote RPC handler.
//
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	// Your code here (2A, 2B).
	if rf.role!=Leader {
		rf.electionTimer.Reset(ElectionTimeout*time.Microsecond)
	}else {
		fmt.Println(args)
		if args.term>rf.currentTerm{
			rf.changeRole(Follower)
		}
	}
	
}

func (rf* Raft) appendEntriesToFollower(idx int)  bool{
	rf.lock("appendEntries")
	toleranceTimer:=time.NewTimer(RpcToleranceTimeOut)
	defer toleranceTimer.Stop()
	args:=&AppendEntriesArgs{}
	reply:=&AppendEntriesReply{}
	args.term=rf.currentTerm
	rf.unlock("appendEntries")
	for  {
		select {
		case <-toleranceTimer.C:
			return false
		default:
			{
				//Call always return, so it will end when tolerance timer goes out
				//or ok is true
				fmt.Printf("server id:%d, server role:%d, term:%d ,target idx:%d\n",rf.me,rf.role,args.term,idx)
				ok := rf.peers[idx].Call("Raft.AppendEntries", args, reply)
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