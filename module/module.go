package module

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
	"strings"
)

func IsInt(dirName string) bool {
	_, err := strconv.ParseInt(dirName, 10, 64)
	if err != nil {
		return false
	}
	return true
}

func GetUptime() (int64, error) {
	var uptime int64

	file, err := os.Open("/rootfs/proc/uptime")
	if err != nil {
		return -1, err
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var tempFloat float64
		splitScan := strings.Split(scanner.Text(), " ")
		tempFloat, err = strconv.ParseFloat(splitScan[0], 64)
		uptime = (int64)(tempFloat)
		if err != nil {
			return -1, err
		}
	}
	if scanner.Err() != nil {
		return -1, err
	}

	return uptime, nil
}

func GetPidList() ([]string, error) {
	var pidList []string

	files, err := ioutil.ReadDir("/rootfs/proc/")
	if err != nil {
		return pidList, err
	}

	for _, file := range files {
		if IsInt(file.Name()) {
			pidList = append(pidList, file.Name())
		}
	}

	return pidList, nil
}

func GetCpuUsage(dirList []string, uptime int64) ([]string, []float32, []uint64, string, error) {
	var pNameList []string
	var cpuUsageList []float32
	var memUsageList []uint64
	var kubeletPid string
	runtime := "docker"

	prefixProc := "/rootfs/proc/"
	postfixProc := "/stat"
	for _, dirName := range dirList {
		statPwd := prefixProc + dirName + postfixProc
		file, err := os.Open(statPwd)
		if err != nil {
			continue
		}

		var readLine string
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			readLine = scanner.Text()
		}
		if scanner.Err() != nil {
			continue
		}

		splitReadLine := strings.Split(readLine, " ")

		if strings.Contains(splitReadLine[1], "kubelet") {
			kubeletPid = splitReadLine[0]
			// can't be err
		}

		utime, err := strconv.ParseFloat(splitReadLine[13], 10)
		if err != nil {
			continue
		}
		stime, err := strconv.ParseFloat(splitReadLine[14], 10)
		if err != nil {
			continue
		}
		cutime, err := strconv.ParseFloat(splitReadLine[15], 10)
		if err != nil {
			continue
		}
		cstime, err := strconv.ParseFloat(splitReadLine[16], 10)
		if err != nil {
			continue
		}
		starttime, err := strconv.ParseFloat(splitReadLine[21], 10)
		if err != nil {
			continue
		}
		memUsage, err := strconv.ParseUint(splitReadLine[22], 10, 64)
		if err != nil {
			continue
		}

		totalTime := utime + stime + cutime + cstime
		seconds := (float64)(uptime) - (starttime / 100)
		cpuUsage := 100.0 * (float32)(((float32)(totalTime)/100.0)/(float32)(seconds))

		//cpuUsageList = append(cpuUsageList, cpuUsage)

		newpName := fmt.Sprintf("%s", splitReadLine[1][1:len(splitReadLine[1])-1])
		pNameList = append(pNameList, newpName)
		cpuUsageList = append(cpuUsageList, cpuUsage)
		memUsageList = append(memUsageList, memUsage)
	}

	statPwd := prefixProc + kubeletPid + "/cmdline"
	file, _ := os.Open(statPwd)

	var readLine string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		readLine = scanner.Text()
	}
	splitedCmdline := strings.Split(readLine, "\x00")

	for _, cmd := range splitedCmdline {
		if strings.Contains(cmd, "container-runtime-endpoint") {
			temp := strings.Split(cmd, "=")
			if strings.Contains(temp[1], "crio") {
				runtime = "crio"
				break
			} else if strings.Contains(temp[1], "containerd") {
				runtime = "containerd"
				break
			}
		}
	}
	return pNameList, cpuUsageList, memUsageList, runtime, nil
}

func GetPidMapper(dirList []string) (map[int]int, error) {
	var pidMap map[int]int
	pidMap = make(map[int]int)

	prefixProc := "/rootfs/proc/"
	postfixProc := "/stat"
	for _, dirName := range dirList {
		statPwd := prefixProc + dirName + postfixProc
		file, err := os.Open(statPwd)
		if err != nil {
			continue
		}

		var readLine string
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			readLine = scanner.Text()
		}
		if scanner.Err() != nil {
			continue
		}

		splitReadLine := strings.Split(readLine, " ")
		pid, err := strconv.ParseInt(splitReadLine[0], 10, 32)
		if err != nil {
			continue
		}
		ppid, err := strconv.ParseInt(splitReadLine[3], 10, 32)
		if err != nil {
			continue
		}

		pidMap[(int)(pid)] = (int)(ppid)
	}

	return pidMap, nil
}

func FindRootPid(pidMap map[int]int, nowPid int) (string, error) {
	var newLine string

	if pidMap[nowPid] == 1 {
		file, err := os.Open("/rootfs/proc/" + strconv.Itoa(nowPid) + "/cmdline")
		if err != nil {
			return "", err
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			newLine = scanner.Text()
		}
		if scanner.Err() != nil {
			return "", scanner.Err()
		}
		if !strings.Contains(newLine, "containerd") && !strings.Contains(newLine, "crio") {
			return "Host", nil
		}

		splitLine := strings.Split(newLine, "\x00")

		for i := 0; i < len(splitLine); i++ {
			if splitLine[i] == "-id" {
				return splitLine[i+1], nil
			}
		}
		return "Host", nil
	} else if pidMap[nowPid] == 2 {
		return "Host", nil
	} else {
		FindRootPid(pidMap, pidMap[nowPid])
	}

	return "Host", nil
}

type JsonAll struct {
	Config      JsonConfig `json:"Config"`
	Annotations JsonConfig `json:"annotations"`
}

type JsonConfig struct {
	Labels      JsonLabels `json:"Labels"`
	SandboxName string     `json:"io.kubernetes.cri.sandbox-name,omitempty"`
	PodName     string     `json:"io.kubernetes.pod.name,omitempty"`
}

type JsonLabels struct {
	PodName string `json:"io.kubernetes.pod.name,omitempty"`
}

func GetContainerId(pidMap map[int]int, runtime string) (map[int]string, error) {
	parameter, contains := func(runtime string) (string, string) {
		if runtime == "crio" {
			return "-c", "cri-o"
		} else {
			return "-id", "containerd-shim"
		}
	}(runtime)

	pidNameMap := make(map[int]string)
	var err error
	for a, _ := range pidMap {
		var whoIsRoot, newLine, containerId string
		nowPid := a
		for {
			err = nil
			if pidMap[nowPid] == 0 {
				whoIsRoot = "Host"
				break
			} else if pidMap[nowPid] == 1 {
				file, innerErr := os.Open("/rootfs/proc/" + strconv.Itoa(nowPid) + "/cmdline")
				if innerErr != nil {
					err = innerErr
					break
				}
				scanner := bufio.NewScanner(file)
				for scanner.Scan() {
					newLine = scanner.Text()
				}
				if scanner.Err() != nil {
					err = innerErr
					break
				}
				if !strings.Contains(newLine, contains) {
					whoIsRoot = "Host"
					err = nil
					break
				}

				//containerId = newLine[strings.Index(newLine, "-id")+4 : strings.Index(newLine, "-address")-1]
				splitNewline := strings.Split(newLine, "\x00")

				for i := 0; i < len(splitNewline); i++ {
					if splitNewline[i] == parameter {
						containerId = splitNewline[i+1]
						break
					}
				}

				whoIsRoot = "Container"
				break
			} else if pidMap[nowPid] == 2 {
				whoIsRoot = "Host"
				err = nil
				break
			}
			if err != nil {
				break
			}
			nowPid = pidMap[nowPid]
		}
		if whoIsRoot == "Container" {
			var newJson string
			var newJsonStruct JsonAll
			var file *os.File

			if runtime == "crio" {
				file, err = os.Open("/rootfs/containers/storage/overlay-containers/" + containerId + "/userdata/config.json")
			} else if runtime == "containerd" {
				file, err = os.Open("/rootfs/k8s.io/" + containerId + "/config.json")
			} else if runtime == "docker" {
				file, err = os.Open("/rootfs/docker/containers/" + containerId + "/config.v2.json")
			}
			if err != nil {
				continue
			}

			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				newJson += scanner.Text()
			}
			if scanner.Err() != nil {
				continue
			}
			err = json.Unmarshal([]byte(newJson), &newJsonStruct)

			// fmt.Println(newJsonStruct)

			if runtime == "crio" {
				whoIsRoot = newJsonStruct.Annotations.PodName + "/" + containerId
			} else if runtime == "containerd" {
				whoIsRoot = newJsonStruct.Annotations.SandboxName + "/" + containerId
			} else if runtime == "docker" {
				whoIsRoot = newJsonStruct.Config.Labels.PodName + "/" + containerId
			}
		}
		pidNameMap[a] = whoIsRoot
	}

	return pidNameMap, nil
}

type JsonSha256 struct {
	MediaType string        `json:"mediaType"`
	ShaConfig JsonShaConfig `json:"config"`
	Layers    []JsonLayers  `json:"layers"`
}

type JsonShaConfig struct {
	Digest string `json:"digest"`
}

type JsonLayers struct {
	Digest string `json:"digest"`
}

type JsonContainerConfig struct {
	ContainerConfig JsonContainerImage `json:"container_config"`
}

type JsonContainerImage struct {
	Image string `json:"Image,omitempty"`
}

func GetParsedSha256() (map[string](map[string]bool), error) {
	layerMap := map[string][]string{}
	imageMap := map[string](map[string]bool){}
	// listMap := map[string][]string{}
	layerChecker := map[string]bool{}
	sha256Prefix := "/rootfs/sha256/"
	files, err := ioutil.ReadDir(sha256Prefix)
	if err != nil {
		return imageMap, err
	}
	for _, file := range files {
		if _, ok := layerMap[file.Name()]; ok {
			continue
		}

		jsonSha256 := JsonSha256{}
		layerList := []string{}
		sha256, err := os.Open(sha256Prefix + file.Name())
		if err != nil {
			return imageMap, err
		}

		gz, err := gzip.NewReader(sha256)
		if err != nil {
			sha256.Seek(0, io.SeekStart)
			var fileContent string
			scanner := bufio.NewScanner(sha256)

			for scanner.Scan() {
				fileContent += scanner.Text()
			}
			if scanner.Err() != nil {
				continue
			}

			err = json.Unmarshal([]byte(fileContent), &jsonSha256)
			if err != nil {
				continue
			}

			if jsonSha256.MediaType != "application/vnd.docker.distribution.manifest.v2+json" {
				continue
			}

			for _, layer := range jsonSha256.Layers {
				layerList = append(layerList, layer.Digest)
				if _, ok := layerChecker[layer.Digest]; !ok {
					layerChecker[layer.Digest] = true
				}
			}

			layerMap[jsonSha256.ShaConfig.Digest] = layerList
		} else {
			continue
		}
		_ = gz
	}

	for key, layers := range layerMap {
		var temp string
		jsonContainer := &JsonContainerConfig{}
		cc, err := os.Open(sha256Prefix + key[7:])
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(cc)
		for scanner.Scan() {
			temp += scanner.Text()
		}
		if scanner.Err() != nil {
			continue
		}

		err = json.Unmarshal([]byte(temp), &jsonContainer)
		if err != nil {
			fmt.Println(err)
			continue
		}

		fileMap := map[string]bool{}

		flag := false

		if len(jsonContainer.ContainerConfig.Image) == 0 {
			continue
		}

		for _, layer := range layers {

			file, err := os.Open(sha256Prefix + layer[7:])
			if err != nil {
				flag = true
				break
			}

			gz, err := gzip.NewReader(file)
			if err != nil {
				continue
			}

			tar := tar.NewReader(gz)

			for {
				header, err := tar.Next()
				if err == io.EOF || err != nil {
					break
				}

				if _, ok := fileMap[header.Name]; !ok {
					fileMap[header.Name] = true
				}
			}
		}
		if flag {
			continue
		}

		imageMap[jsonContainer.ContainerConfig.Image] = fileMap
	}

	return imageMap, nil
}

type JsonState struct {
	ConfigFS JsonConfigFS `json:"config"`
	Root     JsonConfigFS `json:"root"`
}

type JsonConfigFS struct {
	RootFS string `json:"rootfs"`
	Path   string `json:"path"`
}

type JsonContainerd struct {
	Annotations JsonAnno `json:"annotations"`
}

type JsonAnno struct {
	ContainerType string `json:"io.kubernetes.cri.container-type"`
	ImageName     string `json:"io.kubernetes.cri.image-name,omitempty"`
	SandboxId     string `json:"io.kubernetes.cri.sandbox-id"`
}

func GetFileSystemDir(containerIds []string, pidNameMap map[int]string, runtime string) ([]string, []string, error) {
	mergedLayerDirList := make([]string, 0)
	diffLayerDirList := make([]string, 0)
	realContainer := map[string]string{}
	diffLayerMap := map[string]string{}

	if runtime == "containerd" {
		dir := "/rootfs/proc/1/mounts"
		file, err := os.Open(dir)
		if err != nil {
			return mergedLayerDirList, diffLayerDirList, err
		}

		var diffLayerDir, key string

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			newLine := scanner.Text()
			splitNewLine := strings.Split(newLine, " ")
			if splitNewLine[0] == "overlay" {
				tempSplit := strings.Split(splitNewLine[3], ",")
				tempSplit2 := strings.Split(tempSplit[3], "=")[1]
				tempSplit3 := strings.Split(splitNewLine[1], "/")

				key = tempSplit3[5]
				diffLayerDir = strings.Replace(tempSplit2, "/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/", "/rootfs/", 1)
				diffLayerMap[key] = diffLayerDir
			}
		}
		if scanner.Err() != nil {
			return mergedLayerDirList, diffLayerDirList, err
		}

		contPrefix := "/rootfs/k8s.io/"
		contPostfix := "/config.json"
		contIds := make([]string, 0)
		files, err := ioutil.ReadDir(contPrefix)
		if err != nil {
			return mergedLayerDirList, diffLayerDirList, err
		}

		for _, file := range files {
			contIds = append(contIds, file.Name())
		}

		for _, contId := range contIds {
			jsonContainerd := JsonContainerd{}

			var fileContent string
			pwd := contPrefix + contId + contPostfix
			file, err := os.Open(pwd)
			if err != nil {
				continue
			}

			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				fileContent = fileContent + scanner.Text()
			}
			if scanner.Err() != nil {
				continue
			}

			err = json.Unmarshal([]byte(fileContent), &jsonContainerd)

			if err != nil {
				continue
			}

			if jsonContainerd.Annotations.ContainerType == "container" {
				if _, ok := realContainer[jsonContainerd.Annotations.SandboxId]; !ok {
					realContainer[jsonContainerd.Annotations.SandboxId] = contId
					mergedLayerDirList = append(mergedLayerDirList, contPrefix+contId+"/rootfs")
					diffLayerDirList = append(diffLayerDirList, diffLayerMap[contId])
				}
			}
		}
	} else {
		prefixState := func(runtime string) string {
			if runtime == "docker" {
				return "/rootfs/moby/"
			} else {
				return "/rootfs/containers/storage/overlay-containers/"
			}
		}(runtime)
		postfixState := func(runtime string) string {
			if runtime == "docker" {
				return "/state.json"
			} else {
				return "/userdata/config.json"
			}
		}(runtime)

		for _, containerId := range containerIds {
			var newJson, mergedLayerDir string
			var jsonState JsonState

			fullStateDir := prefixState + containerId + postfixState

			file, err := os.Open(fullStateDir)
			if err != nil {
				return mergedLayerDirList, diffLayerDirList, err
			}
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				newJson += scanner.Text()
			}
			if scanner.Err() != nil {
				return mergedLayerDirList, diffLayerDirList, scanner.Err()
			}

			err = json.Unmarshal([]byte(newJson), &jsonState)
			if err != nil {
				return mergedLayerDirList, diffLayerDirList, err
			}
			mergedLayerDir = func(runtime string) string {
				if runtime == "docker" {
					return "/rootfs" + jsonState.ConfigFS.RootFS[strings.Index(jsonState.ConfigFS.RootFS, "/docker"):]
				} else {
					return "/rootfs" + jsonState.Root.Path[strings.Index(jsonState.Root.Path, "/containers"):]
				}
			}(runtime)

			diffLayerDir := strings.Replace(mergedLayerDir, "merged", "diff", 1)

			mergedLayerDirList = append(mergedLayerDirList, mergedLayerDir)
			diffLayerDirList = append(diffLayerDirList, diffLayerDir)
		}
	}

	return mergedLayerDirList, diffLayerDirList, nil
}

var dirWalker DirWalker

func Walk(filePath string, depth int, maxDepth int) error {
	files, err := ioutil.ReadDir(filePath)
	if err != nil {
		return err
	}
	for _, file := range files {
		if file.IsDir() == true && depth < maxDepth {
			dirWalker.FileList = append(dirWalker.FileList, filePath+file.Name()+"/")
			Walk(filePath+file.Name()+"/", depth+1, maxDepth)
		} else if file.IsDir() != true && depth <= maxDepth {
			if file.Mode()&fs.ModeSymlink == fs.ModeSymlink {
				var newDir string
				temp, err := os.Readlink(filePath + file.Name())
				var newStat fs.FileInfo

				if err != nil {
					return err
				}
				if temp[0] == '/' {
					newDir = dirWalker.Root + temp
					newStat, err = os.Stat(newDir)
				} else {
					newDir = filePath + temp
					newStat, err = os.Stat(newDir)
				}
				if err != nil {
					return err
				}

				if newStat.Mode()&fs.ModeDir == fs.ModeDir && depth < maxDepth {
					dirWalker.FileList = append(dirWalker.FileList, newDir+"/")
					Walk(newDir+"/", depth+1, maxDepth)
				} else {
					dirWalker.FileList = append(dirWalker.FileList, newDir)
				}
			} else {
				dirWalker.FileList = append(dirWalker.FileList, filePath+file.Name())
			}
		}
	}
	return nil
}

type MergedList struct {
	PodName     string   `json:"PodName"`
	ContainerId string   `json:"ContainerId"`
	FileList    []string `json:"FileList"`
}

type DirWalker struct {
	Root     string
	FileList []string
}

func GetFileList(podNames []string, containerIds []string, mergedList []string, diffList []string, depth int64, runtime string) (string, error) {
	var jsonMerged []byte
	var tempMergedList []MergedList

	mergedMap := map[string][]int{}
	diffMap := map[string][]int{}

	for i := 0; i < len(mergedList); i++ {
		dirWalker = DirWalker{"", make([]string, 0)}
		merged := mergedList[i]
		containerId := containerIds[i]
		podName := podNames[i]
		var tempMerged MergedList

		tempMerged.PodName = podName
		tempMerged.ContainerId = containerId
		dirWalker.Root = merged
		err := Walk(merged+"/", 0, int(depth))
		if err != nil {
			return string(jsonMerged), err
		}
		for j := 0; j < len(dirWalker.FileList); j++ {
			dirWalker.FileList[j] = dirWalker.FileList[j][len(dirWalker.Root):]
			tempArr := make([]int, 3)
			tempArr[0] = i
			tempArr[1] = j
			mergedMap[dirWalker.FileList[j]] = tempArr
		}
		tempMerged.FileList = dirWalker.FileList

		tempMergedList = append(tempMergedList, tempMerged)
	}

	for i := 0; i < len(diffList); i++ {
		dirWalker = DirWalker{"", make([]string, 0)}
		diff := diffList[i]
		//containerId := containerIds[i]
		//podName := podNames[i]
		//var tempDiff MergedList

		//tempMerged.PodName = podName
		//tempMerged.ContainerId = containerId

		dirWalker.Root = diff
		err := Walk(diff+"/", 0, int(depth))
		if err != nil {
			return string(jsonMerged), err
		}
		for j := 0; j < len(dirWalker.FileList); j++ {
			dirWalker.FileList[j] = dirWalker.FileList[j][len(dirWalker.Root):]
			if tempArr, ok := mergedMap[dirWalker.FileList[j]]; ok {
				if _, ok := diffMap[dirWalker.FileList[j]]; !ok {
					diffMap[dirWalker.FileList[j]] = tempArr
				}
			}
		}
	}

	for _, tempArr := range diffMap {
		tempMergedList[tempArr[0]].FileList[tempArr[1]] = tempMergedList[tempArr[0]].FileList[tempArr[1]] + " MODIFIED"
	}

	jsonMerged, err := json.MarshalIndent(tempMergedList, "", "  ")
	if err != nil {
		return string(jsonMerged), err
	}

	return string(jsonMerged), nil
}

type ProcessInfo struct {
	ProcessName string `json:"ProcessName"`
	CpuUsage    string `json:"CpuUsage"`
	MemUsage    string `json:"MemoryUsage"`
	ProcessId   string `json:"ProcessId"`
	WhoIsParent string `json:"WhoIsParent"`
}

func WriteFile(filePath string, pNameList []string, cpuList []float32, memList []uint64, pidList []string, pidNameMap map[int]string, jsonMerged string) error {
	processInfo := make([]ProcessInfo, 0)
	for i := 0; i < len(pNameList); i++ {
		var temp ProcessInfo
		pid, err := strconv.Atoi(pidList[i])
		if err != nil {
			return err
		}
		temp = ProcessInfo{pNameList[i], fmt.Sprintf("%.3f%%", cpuList[i]), fmt.Sprintf("%dMB", memList[i]/1024/1024), pidList[i], pidNameMap[pid]}
		processInfo = append(processInfo, temp)
	}

	sort.Slice(processInfo, func(i, j int) bool {
		if processInfo[i].CpuUsage > processInfo[j].CpuUsage {
			return true
		} else if processInfo[i].CpuUsage == processInfo[j].CpuUsage {
			return processInfo[i].MemUsage > processInfo[j].MemUsage
		} else {
			return false
		}
	})

	toy, err := json.MarshalIndent(processInfo, "", "  ")
	if err != nil {
		return err
	}

	if _, err := os.Stat(filePath); err != nil {
		err := os.MkdirAll(filePath, 644)
		if err != nil {
			return err
		}
	}

	err = ioutil.WriteFile(filePath+"process", toy, 0644)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(filePath+"container", []byte(jsonMerged), 0644)
	if err != nil {
		return err
	}

	// var newByte []byte
	// var tempString string
	// for i := 0; i < len(processInfo); i++ {
	// 	pid, err := strconv.Atoi(processInfo[i].ProcessId)
	// 	if err != nil {
	// 		continue
	// 	}
	// 	tempString += fmt.Sprintf("%s %6.3f%% %4dMB %s\n", processInfo[i].ProcessName, processInfo[i].CpuUsage, processInfo[i].MemUsage/1024/1024, pidNameMap[pid])
	// }
	// newByte = []byte(tempString)

	// err := ioutil.WriteFile(filePath+"/cpu", newByte, 0644)
	// if err != nil {
	// 	return err
	// }

	return nil
}

func MakeJsonData(pNameList []string, cpuList []float32, memList []uint64, pidList []string, pidNameMap map[int]string) (string, error) {
	processInfo := make([]ProcessInfo, 0)
	for i := 0; i < len(pNameList); i++ {
		var temp ProcessInfo
		pid, err := strconv.Atoi(pidList[i])
		if err != nil {
			return "", err
		}
		temp = ProcessInfo{pNameList[i], fmt.Sprintf("%.3f%%", cpuList[i]), fmt.Sprintf("%dMB", memList[i]/1024/1024), pidList[i], pidNameMap[pid]}
		processInfo = append(processInfo, temp)
	}

	sort.Slice(processInfo, func(i, j int) bool {
		if processInfo[i].CpuUsage > processInfo[j].CpuUsage {
			return true
		} else if processInfo[i].CpuUsage == processInfo[j].CpuUsage {
			return processInfo[i].MemUsage > processInfo[j].MemUsage
		} else {
			return false
		}
	})

	jsonData, err := json.MarshalIndent(processInfo, "", "  ")
	if err != nil {
		return "", err
	}

	return string(jsonData), nil
}

var FileListJsonMerged string
var ProcessInfoJsonData string

func Monitoring() error {
	uptime, err := GetUptime()
	if err != nil {
		panic(err)
	}

	pidList, err := GetPidList()
	if err != nil {
		panic(err)
	}

	pNameList, cpuList, memList, runtime, err := GetCpuUsage(pidList, uptime)
	if err != nil {
		panic(err)
	}

	pidMap, err := GetPidMapper(pidList)
	if err != nil {
		panic(err)
	}

	pidNameMap, err := GetContainerId(pidMap, runtime)
	if err != nil {
		panic(err)
	}

	var ids, pods []string
	containerIds := make(map[string]bool)

	for _, pidName := range pidNameMap {
		if pidName != "Host" {
			temp := strings.Split(pidName, "/")
			if len(temp) > 1 {
				containerId := temp[1]
				if _, ok := containerIds[containerId]; !ok {
					containerIds[containerId] = true
					pods = append(pods, temp[0])
					ids = append(ids, containerId)
				}
			}
		}
	}

	mergedList, diffList, err := GetFileSystemDir(ids, pidNameMap, runtime)
	if err != nil {
		panic(err)
	}

	var depth int64 // depth.........
	depth, err = strconv.ParseInt("3", 10, 32)
	if err != nil {
		panic(err)
	}
	FileListJsonMerged, err = GetFileList(pods, ids, mergedList, diffList, depth, runtime)
	if err != nil {
		panic(err)
	}

	ProcessInfoJsonData, err = MakeJsonData(pNameList, cpuList, memList, pidList, pidNameMap)
	if err != nil {
		panic(err)
	}

	// println(ProcessInfoJsonData, FileListJsonMerged)
	return nil

}
