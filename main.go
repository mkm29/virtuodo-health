package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	virtDir         = "/opt/virtuoso-opensource"
	memoryThreshold = 20
)

// create struct to hold Virtuoso status
// TODO: if you want this to be a livenessProbe, need to add a bool field
type Status struct {
	TotalBuffers       int
	UsedBuffers        int
	DirtyBuffers       int
	FreeBuffers        int
	PercentBuffersFree float64
	MemoryTotal        int
	MemoryUsed         int
	MemoryFree         int
	MemoryCache        int
	MemoryPercentFree  float64
	DiskUsed           int
	DiskFree           int
	PercentDiskUsed    float64
	Status             bool
}

func (s *Status) getVirtuosoStats() (bool, error) {
	// execute OS command to get Virtuoso status
	cmd := exec.Command("isql", "0.0.0.0:1111", "dba", "dba", "status.sql")
	out, err := cmd.Output()
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return false, err
	}

	var bre = regexp.MustCompile(`(\d{1,}) buffers`)
	rs := bre.FindStringSubmatch(string(out))

	s.TotalBuffers, _ = strconv.Atoi(rs[1])
	var bure = regexp.MustCompile(`, (\d{1,}) used`)
	rs2 := bure.FindStringSubmatch(string(out))
	s.UsedBuffers, _ = strconv.Atoi(rs2[1])
	var dbre = regexp.MustCompile(`, (\d{1,}) dirty`)
	rs3 := dbre.FindStringSubmatch(string(out))
	s.DirtyBuffers, _ = strconv.Atoi(rs3[1])
	// compute percent of buffers free
	s.PercentBuffersFree = float64(s.TotalBuffers-s.UsedBuffers) / float64(s.TotalBuffers) * 100
	return true, nil
}

func (s *Status) getDiskUsage() (bool, error) {
	cs := fmt.Sprintf("df -k %s | tr -s ' ' | sed 1d", virtDir)
	out, err := exec.Command("bash", "-c", cs).Output()
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return false, err
	}
	// split by spaces
	df := strings.Split(string(out), " ")

	diskUsed, _ := strconv.Atoi(df[2])
	diskFree, _ := strconv.Atoi(df[3])
	percentDiskUsed := float64(diskUsed) / (float64(diskFree) + float64(diskUsed)) * 100
	s.DiskUsed = diskUsed
	s.DiskFree = diskFree
	s.PercentDiskUsed = percentDiskUsed
	return true, nil
}

func (s *Status) getMemoryUsage() (bool, error) {
	cs := fmt.Sprintf("free | grep 'Mem' | tr -s ' '")
	out, err := exec.Command("bash", "-c", cs).Output()
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return false, err
	}
	// split by spaces
	mem := strings.Split(string(out), " ")
	s.MemoryFree, _ = strconv.Atoi(mem[3])
	s.MemoryUsed, _ = strconv.Atoi(mem[2])
	s.MemoryTotal, _ = strconv.Atoi(mem[1])
	s.MemoryCache, _ = strconv.Atoi(mem[5])
	s.MemoryPercentFree = float64(s.MemoryFree) / float64(s.MemoryTotal) * 100
	return true, nil
}

func getStatus(w http.ResponseWriter, r *http.Request) {
	// print current date
	fmt.Println(time.Now(), " | Getting Virtuoso status")
	w.Header().Set("Content-Type", "application/json")
	var status Status // create struct to hold Virtuoso status

	// get Virtuoso status
	ok, err := status.getVirtuosoStats()
	if !ok {
		fmt.Println(err)
		return
	}

	// DISK USAGE
	ok, err = status.getDiskUsage()
	if !ok {
		fmt.Printf("Error: %s\n", err)
		return
	}
	// MEMORY USAGE
	ok, err = status.getMemoryUsage()
	if !ok {
		fmt.Printf("Error: %s\n", err)
		return
	}
	if status.MemoryPercentFree < memoryThreshold {
		fmt.Printf("Memory usage is %.2f%%\n", status.MemoryPercentFree)
		status.Status = false
	} else {
		status.Status = true
	}

	json.NewEncoder(w).Encode(status)
}

func main() {
	fmt.Printf("Starting Virtuoso status server at %s\n", time.Now())
	http.HandleFunc("/", getStatus)

	err := http.ListenAndServe(":3333", nil)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
	}
}
