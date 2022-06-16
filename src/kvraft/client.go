package kvraft

import (
	"../labrpc"
	"time"
)
import "crypto/rand"
import "math/big"

const (
	reRequestTimeOut =	20*time.Millisecond
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
		DPrintf("client get , client id %d , key %v,cmd id :%v", ck.clientId,key,ck.commandId)

		ok:=ck.servers[leaderId].Call("KVServer.Get", &args, &reply)
		//失败重传?
		if !ok {
			DPrintf("failed to call server\n")
			//try next server
			ck.serverLeaderId=(leaderId+1)% len(ck.servers)
			time.Sleep(reRequestTimeOut)
			continue
		}

		success:=reply.Err

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
			time.Sleep(reRequestTimeOut)
			ck.serverLeaderId=(leaderId+1)% len(ck.servers)
			continue
		}else {
			//try next server
			time.Sleep(reRequestTimeOut)
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
		DPrintf("client put append , client id %d, op %v, key %v, value %v , commandid %v time %v", ck.clientId,op,key,value,ck.commandId,time.Now().Sub(ck.startTime))
		ok:=ck.servers[leaderId].Call("KVServer.PutAppend", &args, &reply)

		if !ok {
			time.Sleep(reRequestTimeOut)
			ck.serverLeaderId=(leaderId+1)%len(ck.servers)
			continue
		}

		err:=reply.Err
		DPrintf("client put append result , client id %d,  command id %v, result %v time %v", ck.clientId,ck.commandId,err,time.Now().Sub(ck.startTime))
		if err==ErrWrongLeader {
			time.Sleep(reRequestTimeOut)
			ck.serverLeaderId=(leaderId+1)%len(ck.servers)
			continue
		}else if err==OK{
			ck.serverLeaderId=leaderId
			ck.commandId++
			break
		}else {
			time.Sleep(reRequestTimeOut)
			ck.serverLeaderId=(leaderId+1)%len(ck.servers)
			continue
		}
	}
}

func (ck *Clerk) Put(key string, value string) {
	ck.PutAppend(key, value, "Put")
}
func (ck *Clerk) Append(key string, value string) {
	ck.PutAppend(key, value, "Append")
}
