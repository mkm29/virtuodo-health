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
	virtDir          = "/opt/virtuoso-opensource"
	memoryThreshold  = 20
	diskThreshold    = 20
	buffersThreshold = 20
)

// create struct to hold Virtuoso status
// TODO: if you want this to be a livenessProbe, need to add a bool field
type Status struct {
	Memory struct {
		Free        int
		Used        int
		Total       int
		Cache       int
		Percent     float64
		PercentFree float64
		Status      bool
	}
	Disk struct {
		Used        int
		Free        int
		Percent     float64
		PercentFree float64
		Status      bool
	}
	Buffers struct {
		Used        int
		Free        int
		Percent     float64
		PercentFree float64
		Status      bool
	}
	Virtuoso struct {
		Buffers struct {
			Total       int
			Used        int
			Free        int
			Dirty       int
			Percent     float64
			PercentFree float64
		}
		Status bool
	}
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
	if len(rs) > 1 {
		s.Virtuoso.Buffers.Total, _ = strconv.Atoi(rs[1])
	}
	var bure = regexp.MustCompile(`, (\d{1,}) used`)
	rs2 := bure.FindStringSubmatch(string(out))
	if len(rs2) > 1 {
		s.Virtuoso.Buffers.Used, _ = strconv.Atoi(rs2[1])
	}
	var dbre = regexp.MustCompile(`, (\d{1,}) dirty`)
	rs3 := dbre.FindStringSubmatch(string(out))
	if len(rs3) > 1 {
		s.Virtuoso.Buffers.Free, _ = strconv.Atoi(rs3[1])
	}
	// if total and free buffers are set, calculate percent free
	if s.Virtuoso.Buffers.Used > 0 && s.Virtuoso.Buffers.Total > 0 {
		s.Virtuoso.Buffers.PercentFree = float64(s.Virtuoso.Buffers.Total-s.Virtuoso.Buffers.Used) / float64(s.Virtuoso.Buffers.Total) * 100
		s.Virtuoso.Buffers.Percent = 100 - s.Virtuoso.Buffers.PercentFree
	}
	s.Virtuoso.Status = s.Virtuoso.Buffers.PercentFree < buffersThreshold
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
	if len(df) < 3 {
		return false, fmt.Errorf("Error: df output has %d fields", len(df))
	}

	diskUsed, _ := strconv.Atoi(df[2])
	diskFree, _ := strconv.Atoi(df[3])
	percentDiskUsed := float64(diskUsed) / (float64(diskFree) + float64(diskUsed)) * 100
	s.Disk.Used = diskUsed
	s.Disk.Free = diskFree
	s.Disk.PercentFree = percentDiskUsed
	s.Disk.Percent = 100 - percentDiskUsed
	s.Disk.Status = s.Disk.Percent > diskThreshold
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
	if len(mem) < 6 {
		fmt.Printf("Error: free output has %d fields: %v", len(mem), mem)
		return false, nil
	}
	s.Memory.Free, _ = strconv.Atoi(mem[3])
	s.Memory.Used, _ = strconv.Atoi(mem[2])
	s.Memory.Total, _ = strconv.Atoi(mem[1])
	s.Memory.Cache, _ = strconv.Atoi(mem[5])
	s.Memory.PercentFree = float64(s.Memory.Free) / float64(s.Memory.Total) * 100
	s.Memory.Status = s.Memory.PercentFree < memoryThreshold
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
	if err != nil {
		fmt.Printf("Disk Error: %s\n", err)
		// return
	}
	// MEMORY USAGE
	ok, err = status.getMemoryUsage()
	if err != nil {
		fmt.Printf("Memory Error: %s\n", err)
		// return
	}
	if status.Memory.PercentFree < memoryThreshold {
		fmt.Printf("Memory usage is %.2f%%\n", status.Memory.PercentFree)
		status.Memory.Status = false
	} else {
		status.Memory.Status = true
	}
	fmt.Printf("Status at %s: %v\n", time.Now(), status)

	json.NewEncoder(w).Encode(status)
}

func main() {
	fmt.Printf("Starting Virtuoso status server at %s\n", time.Now())
	http.HandleFunc("/health", getStatus)

	err := http.ListenAndServe(":3333", nil)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
	}
}
