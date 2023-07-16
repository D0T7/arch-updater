package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type PackageInfo struct {
	PackageName string
	BuildDate   time.Time
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Please provide the number of days as a command-line argument.")
		os.Exit(1)
	}

	outdatedDays, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Println("Invalid number of days. Please provide an integer value.")
		os.Exit(1)
	}

	// Run Pacman to check for updates
	updateInfo, err := exec.Command("pacman", "-Qu").Output()
	if err != nil {
		log.Fatal(err)
	}

	// Extract package names from the update info
	packageNames := make([]string, 0)
	scanner := bufio.NewScanner(strings.NewReader(string(updateInfo)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			packageName := strings.Fields(line)[0]
			packageNames = append(packageNames, packageName)
		}
	}

	// Create a wait group to synchronize goroutines
	var wg sync.WaitGroup

	// Create a channel to receive package information
	pkgInfoChan := make(chan PackageInfo)

	// Start goroutines to fetch package information concurrently
	for _, packageName := range packageNames {
		wg.Add(1)
		go func(pkgName string) {
			defer wg.Done()

			buildDateInfo, err := exec.Command("pacman", "-Si", pkgName).Output()
			if err != nil {
				return
			}

			var buildDate time.Time
			scanner := bufio.NewScanner(strings.NewReader(string(buildDateInfo)))
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if strings.HasPrefix(line, "Build Date") {
					buildDateStr := strings.SplitN(line, ":", 2)[1]
					buildDateStr = strings.TrimSpace(buildDateStr)
					var err error
					buildDate, err = time.Parse("Mon _2 Jan 2006 15:04:05 MST", buildDateStr)
					if err != nil {
						log.Println(err)
						return
					}
					break
				}
			}

			pkgInfo := PackageInfo{
				PackageName: pkgName,
				BuildDate:   buildDate,
			}
			pkgInfoChan <- pkgInfo
		}(packageName)
	}

	// Close the channel once all goroutines finish executing
	go func() {
		wg.Wait()
		close(pkgInfoChan)
	}()

	// Create a list to store outdated packages
	outdatedPackages := make([]string, 0)

	// Process package information from the channel
	for pkgInfo := range pkgInfoChan {
		if time.Since(pkgInfo.BuildDate).Hours()/24 > float64(outdatedDays) {
			outdatedPackages = append(outdatedPackages, pkgInfo.PackageName)
		}
	}

	if len(outdatedPackages) == 0 {
		fmt.Println("No updates found.")
		os.Exit(0)
	}

	// Print the list of packages to be updated
	fmt.Println("Packages to be updated:")
	for _, packageToUpdate := range outdatedPackages {
		fmt.Println(packageToUpdate)
	}

	// Prompt for confirmation before updating
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Do you want to update the packages? (y/n): ")
	confirmation, _ := reader.ReadString('\n')
	if strings.ToLower(strings.TrimSpace(confirmation)) != "y" {
		fmt.Println("Update cancelled.")
		os.Exit(0)
	}

	// Create and run the update command
	updateCommand := append([]string{"sudo", "pacman", "-Sy"}, outdatedPackages...)
	updateCmd := exec.Command(updateCommand[0], updateCommand[1:]...)
	updateCmd.Stdout = os.Stdout
	updateCmd.Stderr = os.Stderr

	err = updateCmd.Run()
	if err != nil {
		log.Fatal(err)
	}

	// Wait for the update command to complete
	err = updateCmd.Wait()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Packages updated successfully.")
}
