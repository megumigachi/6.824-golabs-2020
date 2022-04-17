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
// Task number for each KeyValue emitted by Map.
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
			DPrint("error while requesting")
			os.Exit(1)
		}
		if !tp.OK {
			if tp.Reason ==finished {
				DPrint("Worker exit: all Task finished")
				os.Exit(0)
			}else if tp.Reason ==wait {
				DPrint("Worker: no Task available now, waiting")
				time.Sleep(1*time.Second)
			}
		}else {
			task:=tp.Task
			wk.doTask(task)
		}
	}

}


func(w*worker) doTask(task *Task){
	taskPhase:=task.Phase
	DPrint("worker doing task: taskphase:%d, map:%d, reduce:%d, state:%d ,filename:%s",task.Phase,task.MapNumber,task.ReduceNumber,task.State,task.FileName)
	switch taskPhase{
		case MapPhase :{
			w.doMapTask(task)
		}
		case ReducePhase:{
			w.doReduceTask(task)
		}
	}
	
}

func (w *worker) doMapTask(task *Task) {
	filename:=task.FileName
	file, err := os.Open(filename)
	if err != nil {
		//Todo:report failure
		task.State=Failed
		w.reportTask(task,err)
		return
	}
	content, err := ioutil.ReadAll(file)
	if err != nil {
		//Todo:report failure
		task.State=Failed
		w.reportTask(task, err)
		return
	}
	file.Close()
	kvArray := wk.mapf(filename, string(content))

	nReduce:=task.Nreduce
	reduceArray:=make([][]KeyValue,nReduce)
	for _,kv:=range kvArray {
		hash:=ihash(kv.Key)%nReduce
		reduceArray[hash]=append(reduceArray[hash], kv)
	}
	mapNum:=task.MapNumber
	for idx,rArray:=range reduceArray {
		filename=generateMapFileName(mapNum,idx)
		f,err:=os.Create(filename)
		if err!=nil{
			task.State=Failed
			w.reportTask(task,err)
			return
		}
		enc := json.NewEncoder(f)
		for _, kv := range rArray {
			if err := enc.Encode(&kv); err != nil {
				task.State=Failed
				w.reportTask(task,err)
				return
			}
		}
		if err := f.Close(); err != nil {
			task.State=Failed
			w.reportTask(task,err)
			return
		}
	}
	task.State=Done
	w.reportTask(task,nil)

}

func (w *worker) reportTask(task *Task, err error) {
	if err!=nil {
		DError(err)
	}else {
		DPrint("worker report task: taskphase:%d, map:%d, reduce:%d, state:%d ,filename:%s",task.Phase,task.MapNumber,task.ReduceNumber,task.State,task.FileName)
	}
	ta:=&TaskReportArgs{task}
	tp:=&TaskReportReply{}
	call("Master.ReportTask",ta,tp)
}

func (w *worker) doReduceTask(task *Task) {
	reduceNum:=task.ReduceNumber
	mapNum:=task.MapNumber
	rdMap:=make(map[string][]string)

	for i:=0;i<mapNum ;i++  {
		mapFN:=generateMapFileName(i,reduceNum)
		f,err:= os.Open(mapFN)
		if err!=nil {
			task.State=Failed
			w.reportTask(task,err)
			return
		}
		dec:=json.NewDecoder(f)
		for {
			var kv KeyValue
			if err := dec.Decode(&kv); err != nil {
				break
			}
			rdMap[kv.Key]= append(rdMap[kv.Key], kv.Value)
		}
		if err :=f.Close(); err != nil {
			task.State=Failed
			w.reportTask(task,err)
		}
	}
	rFileName:=generateReduceFileName(reduceNum)
	f,_:= os.Create(rFileName)
	for str,val:=range rdMap {
		rets:=wk.reducef(str,val)
		fmt.Fprintf(f, "%v %v\n", str, rets)
	}
	f.Close()
	task.State=Done
	w.reportTask(task,nil)
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
