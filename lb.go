package main

import (
	"os"
	"fmt"
	"log"
	"time"
	"math"
	"errors"
	"runtime"
	"reflect"
	"github.com/codegangsta/cli"
)

func worker(wid int,
	c *cli.Context,
	rx chan string,
	tx chan Result,
	job Job) {

	num := c.Int("n")
	num_per_worker := int(math.Ceil(float64(num) / float64(c.Int("c"))))
	job.Init(wid, c)
	var result Result
	tx <- result
	<- rx
	if job.GetVerbose() >= 2 {
		log.Printf("worker[%d]: starting job\n", wid)
	}
	result.startTime = time.Now()

	for i := 0; i < num_per_worker; i++ {
		res := job.Request()
		if res {
			job.IncSuccess()
		}
		job.IncCount()
	}
	result.endTime = time.Now()
	job.Finish()
	result.elapsedTime = result.endTime.Sub(result.startTime).Seconds()
	result.wid = wid
	result.count = job.GetCount()
	result.success = job.GetSuccess()
	tx <- result
}

func waitReady(ch chan Result, n int){
	for i := 0; i < n; i++ {
		<- ch
	}
}

func waitResult(ch chan Result, n int) []Result {
	results := make([]Result, n)
	for i := 0; i < n; i++ {
		result := <- ch
		results[result.wid] = result
	}
	return results
}

func reportResult(ctx *cli.Context, results []Result) {
	var firstTime time.Time
	var lastTime time.Time
	var totalRequest int
	var successRequest int
	for i := range results {
		rpq := float64(results[i].count) / results[i].elapsedTime
		if ctx.Int("v") >= 2 {
			log.Printf("worker[%d]: %.2f [#/sec] time=%.3f\n",
				results[i].wid, rpq, results[i].elapsedTime)
		}

		totalRequest += results[i].count
		successRequest += results[i].success
		if firstTime.IsZero() || firstTime.After(results[i].startTime) {
			firstTime = results[i].startTime
		}
		if lastTime.Before(results[i].endTime) {
			lastTime = results[i].endTime
		}
	}
	takenTime := lastTime.Sub(firstTime).Seconds()
	rpq := float64(totalRequest) / takenTime
	concurrency := ctx.Int("c")

	fmt.Printf("Concurrency Level: %d\n", concurrency)
	fmt.Printf("Total Requests: %d\n", totalRequest)
	fmt.Printf("Success Requests: %d\n", successRequest)
	fmt.Printf("Success Rate: %d%%\n", successRequest / totalRequest * 100)
	fmt.Printf("Time taken for tests: %.3f seconds\n", takenTime)
	fmt.Printf("Requests per second: %.2f [#/sec] (mean)\n", rpq)
	fmt.Printf("Time per request: %.3f [ms] (mean)\n",
		float64(concurrency) * takenTime * 1000 / float64(totalRequest))
	fmt.Printf("Time per request: %.3f [ms] " +
		"(mean, across all concurrent requests)\n",
		takenTime * 1000 / float64(totalRequest))
	fmt.Printf("CPU Number: %d\n", runtime.NumCPU())
	fmt.Printf("GOMAXPROCS: %d\n", runtime.GOMAXPROCS(0))
}

func checkArgs(c *cli.Context) error {
	if len(c.Args()) < 1 {
		cli.ShowAppHelp(c)
		return errors.New("few args")
	}

	fmt.Printf("This is LDAPBench, Version %s\n", c.App.Version)
	fmt.Printf("This software is released under the MIT License.\n")
	fmt.Printf("\n")
	fmt.Printf("checkArgs: %+v\n", c.Command.Name)
	return nil
}

func runBenchmark(c *cli.Context, jobType reflect.Type) {
	fmt.Printf("%s Benchmarking: %s\n",
		jobType.Name(), c.Args().First())

	workerNum := c.Int("c");
	tx := make(chan string)
	rx := make(chan Result)

	for i := 0; i < workerNum; i++ {
		job := reflect.New(jobType).Interface().(Job)
		go worker(i, c, tx, rx, job)
	}
	waitReady(rx, workerNum)
	// all worker are ready
	for i := 0; i < workerNum; i++ {
		tx <- "start"
	}
	results := waitResult(rx, workerNum)
	reportResult(c, results)
}

var commonFlags = []cli.Flag {
	cli.IntFlag {
		Name: "verbose, v",
		Value: 0,
		Usage: "How much troubleshooting info to print",
	},
	cli.IntFlag {
		Name: "n",
		Value: 1,
		Usage: "Number of requests to perform",
	},
	cli.IntFlag {
		Name: "c",
		Value: 1,
		Usage: "Number of multiple requests to make",
	},
	cli.StringFlag {
		Name: "D",
		Value: "cn=Manager,dc=example,dc=com",
		Usage: "Bind DN",
	},
	cli.StringFlag {
		Name: "w",
		Value: "secret",
		Usage: "Bind Secret",
	},
	cli.StringFlag {
		Name: "b",
		Value: "dc=example,dc=com",
		Usage: "BaseDN",
	},
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	app := cli.NewApp()
	app.Name = "lb"
	app.Usage = "LDAP Benchmarking Tool"
	app.Version = "0.1.2"
	app.Author = "HAMANO Tsukasa"
	app.Email = "hamano@osstech.co.jp"
	app.Commands = []cli.Command{
		{
			Name: "bind",
			Usage: "LDAP BIND Test",
			Before: checkArgs,
			Action: Bind,
			Flags: commonFlags,
		},
		{
			Name: "add",
			Usage: "LDAP ADD Test",
			Before: checkArgs,
			Action: add,
			Flags: commonFlags,
		},
		{
			Name: "setup",
			Usage: "Add Base Entry",
			Before: checkArgs,
			Action: setup,
			Flags: commonFlags,
		},
	}
	app.Run(os.Args)
}