package mr

import (
	"fmt"
	"os"
)
import "log"
import "net/rpc"
import "hash/fnv"

//
// Map functions return a slice of KeyValue.
//
type KeyValue struct {
	Key   string
	Value string
}

//
// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
//
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}

type worker struct {
	mapf    func(string, string) []KeyValue
	reducef func(string, []string) string
}

var wk *worker

//
// main/mrworker.go calls this function.
//
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {
	//initworker
	wk = &worker{}
	wk.mapf = mapf;
	wk.reducef = reducef;
	for {
		ta:=&TaskRequestArgs{}
		tp:=&TaskRequestReply{}
		ok:=call("Master.RequestTask",ta,tp)
		if !ok {
			DPrint("error while requesting")
			os.Exit(1)
		}
		if !tp.ok {

		}

	}

	// Your worker implementation here.

	// uncomment to send the Example RPC to the master.
	// CallExample()

}


//
// send an RPC request to the master, wait for the response.
// usually returns true.
// returns false if something goes wrong.
//
func call(rpcname string, args interface{}, reply interface{}) bool {
	// c, err := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
	sockname := masterSock()
	c, err := rpc.DialHTTP("unix", sockname)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}
