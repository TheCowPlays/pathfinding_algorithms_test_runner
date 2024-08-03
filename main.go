package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"strconv"
	"sync"
	"time"

	"pathfinding_algorithms_test_runner/algorithms"
	"pathfinding_algorithms_test_runner/maze"
)

type Metrics struct {
	Time              []float64
	VisitedNodes      []int
	VisitedPercentage []float64
	PathLength        []int
	MemoryUsed        []float64
}

var algorithmsMap = map[string]algorithms.Algorithm{
	"dijkstra":     algorithms.Dijkstra{},
	"astar":        algorithms.Astar{},
	"bfs":          algorithms.BFS{},
	"dfs":          algorithms.DFS{},
	"wallFollower": algorithms.WallFollower{},
}

func main() {
	nFlag := flag.String("n", "", "Optional filename marker")
	flag.Parse()

	// Open files for CPU and memory profiling
	cpuProfile, err := os.Create("cpu.prof")
	if err != nil {
		log.Fatal("could not create CPU profile: ", err)
	}
	defer cpuProfile.Close()

	memProfile, err := os.Create("mem.prof")
	if err != nil {
		log.Fatal("could not create memory profile: ", err)
	}
	defer memProfile.Close()

	// Start CPU profiling
	if err := pprof.StartCPUProfile(cpuProfile); err != nil {
		log.Fatal("could not start CPU profile: ", err)
	}
	defer pprof.StopCPUProfile()

	args := flag.Args()
	if len(args) < 2 {
		runTestsWithIncreasingSize(*nFlag)
	} else {
		mazeSize, _ := strconv.Atoi(args[0])
		numTests, _ := strconv.Atoi(args[1])
		var marker string
		if len(args) > 2 {
			marker = args[2]
		}
		runTest(mazeSize, numTests, marker)
	}

	// Stop CPU profiling and write memory profile
	pprof.StopCPUProfile()
	if err := pprof.WriteHeapProfile(memProfile); err != nil {
		log.Fatal("could not write memory profile: ", err)
	}

}

func runTestsWithIncreasingSize(marker string) {
	size := 25
	for {
		fmt.Printf("Running tests with maze size %d\n", size)
		err := runTest(size, 10, marker)
		if err != nil {
			fmt.Printf("Test failed for maze size %d: %s\n", size, err.Error())
			break
		}
		size += 25
	}
}

func runTest(mazeSize, numTests int, marker string) error {
	numRows := mazeSize
	numCols := mazeSize
	metricsSPOn := initializeMetrics()
	metricsSPOff := initializeMetrics()

	// Test mazes with a single path
	for i := 0; i < numTests; i++ {
		grids, startNodes, endNodes := getInitialGrid(numRows, numCols, true)
		var wg sync.WaitGroup
		for algorithm := range algorithmsMap {
			wg.Add(1)
			go func(algorithm string) {
				defer wg.Done()
				runAlgorithm(algorithm, grids[algorithm], startNodes[algorithm], endNodes[algorithm], metricsSPOn)
			}(algorithm)
		}
		wg.Wait()
		fmt.Printf("Completed test %d of %d for mazes with a single path, for size: %d\n", i+1, numTests, mazeSize)
		clearMemory(grids, startNodes, endNodes)
	}

	// Test mazes with multiple paths
	for i := 0; i < numTests; i++ {
		grids, startNodes, endNodes := getInitialGrid(numRows, numCols, false)
		var wg sync.WaitGroup
		for algorithm := range algorithmsMap {
			wg.Add(1)
			go func(algorithm string) {
				defer wg.Done()
				runAlgorithm(algorithm, grids[algorithm], startNodes[algorithm], endNodes[algorithm], metricsSPOff)
			}(algorithm)
		}
		wg.Wait()
		fmt.Printf("Completed test %d of %d for mazes with multiple paths, for size: %d\n", i+1, numTests, mazeSize)
		clearMemory(grids, startNodes, endNodes)
	}

	averagesSPOn := calculateAverages(metricsSPOn)
	averagesSPOff := calculateAverages(metricsSPOff)

	filename := fmt.Sprintf("./data/averages%dx%dx%d.csv", numRows, numCols, numTests)
	if marker != "" {
		filename = fmt.Sprintf("./data/averages%dx%dx%dx%s.csv", numRows, numCols, numTests, marker)
	}
	writeResultsToCsv(filename, averagesSPOn, averagesSPOff)

	return nil
}

func initializeMetrics() map[string]*Metrics {
	return map[string]*Metrics{
		"dijkstra":     {},
		"astar":        {},
		"bfs":          {},
		"dfs":          {},
		"wallFollower": {},
	}
}

func runAlgorithm(algorithm string, grid [][]maze.Node, startNode *maze.Node, endNode *maze.Node, metrics map[string]*Metrics) {
	startTime := time.Now()
	initialMemoryUsage := runtime.MemStats{}
	runtime.ReadMemStats(&initialMemoryUsage)
	visitedNodesInOrder := algorithmsMap[algorithm].FindPath(grid, startNode, endNode)

	finalMemoryUsage := runtime.MemStats{}
	runtime.ReadMemStats(&finalMemoryUsage)
	endTime := time.Now()
	nodesInShortestPathOrder := getNodesInShortestPathOrder(endNode)
	timeTaken := endTime.Sub(startTime).Nanoseconds() // Convert to nanoseconds

	memoryUsed := float64(finalMemoryUsage.HeapAlloc-initialMemoryUsage.HeapAlloc) / (1024 * 1024) // Convert to MB

	totalNodes := len(grid) * len(grid[0])
	wallNodes := countWallNodes(grid)
	nonWallNodes := totalNodes - wallNodes
	visitedPercentage := (float64(len(visitedNodesInOrder)) / float64(nonWallNodes)) * 100

	metrics[algorithm].Time = append(metrics[algorithm].Time, float64(timeTaken))
	metrics[algorithm].VisitedNodes = append(metrics[algorithm].VisitedNodes, len(visitedNodesInOrder))
	metrics[algorithm].VisitedPercentage = append(metrics[algorithm].VisitedPercentage, visitedPercentage)
	metrics[algorithm].PathLength = append(metrics[algorithm].PathLength, len(nodesInShortestPathOrder))
	metrics[algorithm].MemoryUsed = append(metrics[algorithm].MemoryUsed, memoryUsed)
}

func getNodesInShortestPathOrder(endNode *maze.Node) []*maze.Node {
	var nodesInShortestPathOrder []*maze.Node
	currentNode := endNode
	for currentNode != nil {
		nodesInShortestPathOrder = append([]*maze.Node{currentNode}, nodesInShortestPathOrder...)
		currentNode = currentNode.PreviousNode
	}
	return nodesInShortestPathOrder
}

func countWallNodes(grid [][]maze.Node) int {
	count := 0
	for _, row := range grid {
		for _, node := range row {
			if node.IsWall {
				count++
			}
		}
	}
	return count
}

func calculateAverages(metrics map[string]*Metrics) map[string]map[string]float64 {
	averages := make(map[string]map[string]float64)
	for algorithm, metric := range metrics {
		averages[algorithm] = make(map[string]float64)
		numTests := len(metric.Time)
		for _, time := range metric.Time {
			averages[algorithm]["time"] += time
		}
		for _, visitedNodes := range metric.VisitedNodes {
			averages[algorithm]["visitedNodes"] += float64(visitedNodes)
		}
		for _, visitedPercentage := range metric.VisitedPercentage {
			averages[algorithm]["visitedPercentage"] += visitedPercentage
		}
		for _, pathLength := range metric.PathLength {
			averages[algorithm]["pathLength"] += float64(pathLength)
		}
		for _, memoryUsed := range metric.MemoryUsed {
			averages[algorithm]["memoryUsed"] += memoryUsed
		}
		for key := range averages[algorithm] {
			averages[algorithm][key] /= float64(numTests)
		}
	}
	return averages
}

func writeResultsToCsv(filename string, averagesSPOn, averagesSPOff map[string]map[string]float64) {
	file, err := os.Create(filename)
	if err != nil {
		log.Fatalf("Failed to create file: %s", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := []string{"Algorithm", "SinglePath", "Time [ms]", "VisitedNodes", "VisitedPercentage [%]", "PathLength", "MemoryUsed [MB]"}
	if err := writer.Write(header); err != nil {
		log.Fatalf("Failed to write header: %s", err)
	}

	algorithmOrder := []string{"dijkstra", "astar", "bfs", "dfs", "wallFollower"}

	for _, algorithm := range algorithmOrder {
		if metrics, exists := averagesSPOn[algorithm]; exists {
			row := []string{
				algorithm,
				"true",
				fmt.Sprintf("%.3f", metrics["time"]/1e6), // Convert to milliseconds with 3 decimal places
				fmt.Sprintf("%.0f", metrics["visitedNodes"]),
				fmt.Sprintf("%.2f", metrics["visitedPercentage"]),
				fmt.Sprintf("%.0f", metrics["pathLength"]),
				fmt.Sprintf("%.2f", metrics["memoryUsed"]),
			}
			if err := writer.Write(row); err != nil {
				log.Fatalf("Failed to write row for %s: %s", algorithm, err)
			}
		}
	}

	for _, algorithm := range algorithmOrder {
		if metrics, exists := averagesSPOff[algorithm]; exists {
			row := []string{
				algorithm,
				"false",
				fmt.Sprintf("%.3f", metrics["time"]/1e6), // Convert to milliseconds with 3 decimal places
				fmt.Sprintf("%.0f", metrics["visitedNodes"]),
				fmt.Sprintf("%.2f", metrics["visitedPercentage"]),
				fmt.Sprintf("%.0f", metrics["pathLength"]),
				fmt.Sprintf("%.2f", metrics["memoryUsed"]),
			}
			if err := writer.Write(row); err != nil {
				log.Fatalf("Failed to write row for %s: %s", algorithm, err)
			}
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		log.Fatalf("Error flushing writer: %s", err)
	}
}

func getInitialGrid(numRows, numCols int, singlePath bool) (map[string][][]maze.Node, map[string]*maze.Node, map[string]*maze.Node) {
	grids := make(map[string][][]maze.Node)
	startNodes := make(map[string]*maze.Node)
	endNodes := make(map[string]*maze.Node)

	mazeData := maze.GenerateMaze(numRows, numCols, singlePath)

	grids["dijkstra"] = mazeData["gridDijkstra"].([][]maze.Node)
	grids["astar"] = mazeData["gridAstar"].([][]maze.Node)
	grids["bfs"] = mazeData["gridBFS"].([][]maze.Node)
	grids["dfs"] = mazeData["gridDFS"].([][]maze.Node)
	grids["wallFollower"] = mazeData["gridWallFollower"].([][]maze.Node)

	startNodes["dijkstra"] = mazeData["gridDijkstraStartNode"].(*maze.Node)
	startNodes["astar"] = mazeData["gridAstarStartNode"].(*maze.Node)
	startNodes["bfs"] = mazeData["gridBFSStartNode"].(*maze.Node)
	startNodes["dfs"] = mazeData["gridDFSStartNode"].(*maze.Node)
	startNodes["wallFollower"] = mazeData["gridWallFollowerStartNode"].(*maze.Node)

	endNodes["dijkstra"] = mazeData["gridDijkstraEndNode"].(*maze.Node)
	endNodes["astar"] = mazeData["gridAstarEndNode"].(*maze.Node)
	endNodes["bfs"] = mazeData["gridBFSEndNode"].(*maze.Node)
	endNodes["dfs"] = mazeData["gridDFSEndNode"].(*maze.Node)
	endNodes["wallFollower"] = mazeData["gridWallFollowerEndNode"].(*maze.Node)

	return grids, startNodes, endNodes
}

func clearMemory(grids map[string][][]maze.Node, startNodes, endNodes map[string]*maze.Node) {
	for k := range grids {
		grids[k] = nil
	}
	for k := range startNodes {
		startNodes[k] = nil
	}
	for k := range endNodes {
		endNodes[k] = nil
	}
}
