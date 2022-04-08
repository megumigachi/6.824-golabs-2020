package mr

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"time"
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
		//require tasks
		ta:=&TaskRequestArgs{}
		tp:=&TaskRequestReply{}
		ok:=call("Master.RequestTask",ta,tp)
		if !ok {
			//master error?
			DPrint("error while requesting")
			os.Exit(1)
		}
		if !tp.ok {
			if tp.reason==finished {
				DPrint("Worker exit: all task finished")
				os.Exit(0)
			}else if tp.reason==wait {
				DPrint("Worker: no task available now, waiting")
				time.Sleep(1*time.Second)
			}
		}else {
			task:=tp.task
			wk.doTask(task)
		}

	}

	// Your worker implementation here.

	// uncomment to send the Example RPC to the master.
	// CallExample()

}


func(w*worker) doTask(task *Task){
	taskPhase:=task.Phase
	switch taskPhase{
		case MapPhase :{
			w.doMapTask(task)
		}
		case ReducePhase:{
			
		}
	}
	
}

func (w *worker) doMapTask(task *Task) {
	filename:=task.FileName
	file, err := os.Open(filename)
	if err != nil {
		//Todo:report failure
		task.State=Failed
		w.reportTask(task)
		DPrint("cannot open %v", filename)
	}
	content, err := ioutil.ReadAll(file)
	if err != nil {
		//Todo:report failure
		task.State=Failed
		w.reportTask(task)
		DPrint("cannot read %v", filename)

	}
	file.Close()
	kvArray := wk.mapf(filename, string(content))

	//write into tempfile
	nReduce:=task.Nreduce
	var encoder *json.Encoder
	for _,kv:=range kvArray {
		k:=kv.Key
		reduceNum:=ihash(k)%nReduce
		tempFileName:=generateMapFileName(task.MapNumber,reduceNum)
		file, err = os.Open(tempFileName)
		if err != nil {
			task.State=Failed
			w.reportTask(task)
			DPrint("cannot open %v", filename)
		}
		encoder=json.NewEncoder(file)
		encoder.Encode(&kv)
		file.Close()
	}

}

func (w *worker) reportTask(task *Task) {

}

func generateMapFileName(mapNum int,reduceNum int) string {
	return fmt.Sprintf("mr-tmp-%d-%d",mapNum,reduceNum)
}

func generateReduceFileName(reduceNum int) string{
	return fmt.Sprintf("mr-out-%d",reduceNum)
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
