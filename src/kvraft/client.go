package kvraft

import (
	"../labrpc"
	"time"
)
import "crypto/rand"
import "math/big"

const (
	RequestIntervel =	20*time.Millisecond
)

type Clerk struct {
	servers []*labrpc.ClientEnd
	// You will have to modify this struct.
	clientId int64
	commandId int

	lockName string
	//暂存的领导id
	serverLeaderId int

	startTime time.Time
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
	ck.startTime=time.Now()

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
	//fmt.Println("starting getting")

	args:=GetArgs{
		key,
		ck.clientId,
		ck.commandId,
	}

	//客户端，应该不需要加锁
	//客户端的命令是线性执行的，并发毫无意义
	for  {
		reply:=GetReply{
			Err:   "",
			Value: "",
		}
		leaderId:=ck.serverLeaderId
		DPrintf("[client %d][get][command id %d][key %v]", ck.clientId,ck.commandId,key)

		ok:=ck.servers[leaderId].Call("KVServer.Get", &args, &reply)
		//失败重传?
		if !ok {
			DPrintf("[client %d][get failed to call server][command id %d]",ck.clientId,ck.commandId)
			//try next server
			ck.serverLeaderId=(leaderId+1)% len(ck.servers)
			time.Sleep(RequestIntervel)
			continue
		}


		success:=reply.Err
		//DPrintf("client get result , client id %d,  command id %v, result %v time %v", ck.clientId,ck.commandId,success,time.Now().Sub(ck.startTime))

		if success==OK {
			ck.serverLeaderId=leaderId
			ck.commandId++
			return reply.Value
		}else if success==ErrNoKey {
			ck.serverLeaderId=leaderId
			ck.commandId++
			break
		}else if success==ErrWrongLeader {
			//try next server
			time.Sleep(RequestIntervel)
			ck.serverLeaderId=(leaderId+1)% len(ck.servers)
			continue
		}else if success==ErrTimeOut{
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

	args:=PutAppendArgs{
		Key:       key,
		Value:     value,
		Op:        op,
		ClientId:  ck.clientId,
		CommandId: ck.commandId,
	}


	for  {
		reply:=PutAppendReply{Err:""}

		leaderId:=ck.serverLeaderId
		DPrintf("[client %d][write][command id %d][key %v][value %v]", ck.clientId,ck.commandId,key,value)
		ok:=ck.servers[leaderId].Call("KVServer.PutAppend", &args, &reply)

		if !ok {
			time.Sleep(RequestIntervel)
			ck.serverLeaderId=(leaderId+1)%len(ck.servers)
			continue
		}

		err:=reply.Err
		DPrintf("[client %d][write result][command id %d][key %v][value %v]", ck.clientId,ck.commandId,key,value)
		if err==ErrWrongLeader {
			time.Sleep(RequestIntervel)
			ck.serverLeaderId=(leaderId+1)%len(ck.servers)
			continue
		}else if err==OK{
			ck.serverLeaderId=leaderId
			ck.commandId++
			break
		}else if err==ErrTimeOut{
			continue
		}
	}
}

func (ck *Clerk) Put(key string, value string) {
	ck.PutAppend(key, value, OP_Put)
}
func (ck *Clerk) Append(key string, value string) {
	ck.PutAppend(key, value, OP_Append)
}
