package kvraft

import (
	"../labrpc"
	"time"
)
import "crypto/rand"
import "math/big"

const (
	timeOut  =	100*time.Millisecond
)

type Clerk struct {
	servers []*labrpc.ClientEnd
	// You will have to modify this struct.
	clientId int64
	commandId int

	lockName string
	//暂存的领导id
	serverLeaderId int
}



func nrand() int64 {
	max := big.NewInt(int64(1) << 62)
	bigx, _ := rand.Int(rand.Reader, max)
	x := bigx.Int64()
	return x
}

func MakeClerk(servers []*labrpc.ClientEnd) *Clerk {
	ck := new(Clerk)
	ck.servers = servers
	ck.clientId=nrand()
	ck.commandId=0
	ck.serverLeaderId=0
	// You'll have to add code here.
	return ck
}

//
// fetch the current value for a key.
// returns "" if the key does not exist.
// keeps trying forever in the face of all other errors.
//
// you can send an RPC with code like this:
// ok := ck.servers[i].Call("KVServer.Get", &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
//
func (ck *Clerk) Get(key string) string {

	args:=GetArgs{
		key,
		ck.clientId,
		ck.commandId,
	}
	reply:=GetReply{
		Err:   "",
		Value: "",
	}
	//客户端，应该不需要加锁
	//客户端的命令是线性执行的，并发毫无意义
	for  {
		leaderId:=ck.serverLeaderId
		ok:=ck.servers[leaderId].Call("KVServer.Get", &args, &reply)
		//失败重传?
		if !ok {
			DPrintf("failed to call server\n")
			//try next server
			leaderId=(leaderId+1)% len(ck.servers)
			time.Sleep(timeOut)
			continue
		}

		success:=reply.Err

		if success==OK {
			ck.serverLeaderId=leaderId
			return reply.Value
		}else if success==ErrNoKey {
			ck.serverLeaderId=leaderId
			break
		}else if success==ErrWrongLeader {
			//try next server
			ck.serverLeaderId=(leaderId+1)% len(ck.servers)
			continue
		}
	}

	return ""
}

//
// shared by Put and Append.
//
// you can send an RPC with code like this:
// ok := ck.servers[i].Call("KVServer.PutAppend", &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
//
func (ck *Clerk) PutAppend(key string, value string, op string) {
	
}

func (ck *Clerk) Put(key string, value string) {
	ck.PutAppend(key, value, "Put")
}
func (ck *Clerk) Append(key string, value string) {
	ck.PutAppend(key, value, "Append")
}
