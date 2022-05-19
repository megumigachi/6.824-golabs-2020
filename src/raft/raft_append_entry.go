package raft


//
// example RequestVote RPC arguments structure.
// field names must start with capital letters!
//
type AppendEntriesArgs struct {

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
func (rf *Raft) AppendEntries(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
}

func (rf* Raft) appendEntriesToFollower(idx int)  {

}