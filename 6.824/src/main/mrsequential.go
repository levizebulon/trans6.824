package main

//
// simple sequential MapReduce.
//
// go run mrsequential.go wc.so pg*.txt
//

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"plugin"
	"sort"

	"../mr"
)

// for sorting by key.
type ByKey []mr.KeyValue

// for sorting by key.
func (a ByKey) Len() int           { return len(a) }
func (a ByKey) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByKey) Less(i, j int) bool { return a[i].Key < a[j].Key }

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: mrsequential xxx.so inputfiles...\n")
		os.Exit(1)
	}

	mapf, reducef := loadPlugin(os.Args[1])

	//
	// read each input file,
	// pass it to Map,
	// accumulate the intermediate Map output.
	//
	intermediate := []mr.KeyValue{}
	for _, filename := range os.Args[2:] {
		// func Open(name string) (file *File, err error)
		//  Open opens the named file for reading. If successful, methods on the returned file can be used for reading; the associated file descriptor has mode O_RDONLY. If there is an error, it will be of type *PathError.
		file, err := os.Open(filename)
		if err != nil {
			log.Fatalf("cannot open %v", filename)
		}
		// func ReadAll(r io.Reader) ([]byte, error)
		//  ReadAll 从 r 中读取，直至遇到错误或EOF，然后返回它所读取的数据。 一次成功的调用应当返回 err == nil，而非 err == EOF 因为 ReadAll 被定义为从 src 进行读取直至遇到EOF，它并不会将来自 Read 的EOF视作错误来报告。
		content, err := ioutil.ReadAll(file)
		if err != nil {
			log.Fatalf("cannot read %v", filename)
		}
		file.Close()
		kva := mapf(filename, string(content))
		intermediate = append(intermediate, kva...)
	}

	//
	// a big difference from real MapReduce is that all the
	// intermediate data is in one place, intermediate[],
	// rather than being partitioned into NxM buckets.
	//

	sort.Sort(ByKey(intermediate))

	oname := "mr-out-0"
	ofile, _ := os.Create(oname)

	//
	// call Reduce on each distinct key in intermediate[],
	// and print the result to mr-out-0.
	//
	i := 0
	for i < len(intermediate) {
		j := i + 1
		// 这里进行了一个group操作，把键相同的放在一起
		for j < len(intermediate) && intermediate[j].Key == intermediate[i].Key {
			j++
		}
		values := []string{}
		for k := i; k < j; k++ {
			// 把建相同的一组value放在一个数组中
			values = append(values, intermediate[k].Value)
		}
		output := reducef(intermediate[i].Key, values)

		// this is the correct format for each line of Reduce output.
		fmt.Fprintf(ofile, "%v %v\n", intermediate[i].Key, output)

		i = j
	}

	ofile.Close()
}

//
// load the application Map and Reduce functions
// from a plugin file, e.g. ../mrapps/wc.so
//
func loadPlugin(filename string) (func(string, string) []mr.KeyValue, func(string, []string) string) {
	// func Open(path string) (*Plugin, error)
	//  Open opens a Go plugin. If a path has already been opened, then the existing *Plugin is returned. It is safe for concurrent use by multiple goroutines.
	p, err := plugin.Open(filename)
	if err != nil {
		log.Fatalf("cannot load plugin %v", filename)
	}
	// func (p *Plugin) Lookup(symName string) (Symbol, error)
	//  Lookup searches for a symbol named symName in plugin p. A symbol is any exported variable or function. It reports an error if the symbol is not found. It is safe for concurrent use by multiple goroutines.
	xmapf, err := p.Lookup("Map")
	if err != nil {
		log.Fatalf("cannot find Map in %v", filename)
	}
	// 类型断言Type assertions
	// x.(T) 检查x的动态类型是否是T,若成功则提取出x的具体值。如果检查失败则panic。
	mapf := xmapf.(func(string, string) []mr.KeyValue)
	xreducef, err := p.Lookup("Reduce")
	if err != nil {
		log.Fatalf("cannot find Reduce in %v", filename)
	}
	reducef := xreducef.(func(string, []string) string)

	return mapf, reducef
}
