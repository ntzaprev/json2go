package main

import (
	"fmt"
	"os"
	"io/ioutil"
	"encoding/json"
    "strings"
    "path/filepath"
    "sort"
	"log"
	"path"
	"net"
	)
	
type consulServices struct{
	Datacenters [] string `json:"datacenters"`
	Services [] service `json:"services"`
}

type service struct{
	Name string `json:"Name"`
	Tags [] string `json:"Tags"`
	Instances [] instance`json:"Instances"`	
} 

type instance struct{
    Id string `json:"Id"`
	Dc string `json:"Dc"`
	Tags [] string `json:"Tags"`
	Address string `json:"Address"`
	Port string `json:"Port"`
	Status string `json:"Status"`
}

//String implementing string interface
func (t instance) String() string {return fmt.Sprintf("%s_%s",t.Address,t.Port)}

type byString [] instance

//Len of sort interface
func (t byString) Len() int { return len(t) }
//Swap of sort interface
func (t byString) Swap(i,j int) { t[i],t[j] = t[j], t[i]}
//Less of sort interface
func (t byString) Less(i,j int) bool {return strings.Compare(t[i].String() , t[j].String()) < 0}

type servicePattern struct{
    DcPattern string
    TagsPattern [] string
}

func main (){
    
	fmt.Printf("Printing arguments...\n")
	for n,s := range os.Args[1:]{
		fmt.Printf("[%d]\t%s\n", n,s)
	}
    
    lservices := make(map[string] *service)
	cs := readServices()
	
    
    for _,s := range cs.Services{ 
        p, ok := lservices[s.Name]          
        if ok {
           p.Tags = append(p.Tags, s.Tags...)
           p.Instances = append(p.Instances, s.Instances...)
           //fmt.Printf("Dup Service: %s:\nTags: %v\nInstances:%v\n",s.Name,  services[s.Name].Tags, services[s.Name].Instances)
        } else {         
           //Copy the struct or we get into a sea of trouble
           ls := s
           lservices[s.Name] = &ls   
           //fmt.Printf("Service: %s:\nTags: %v\nInstances:%v\n",s.Name,  services[s.Name].Tags, services[s.Name].Instances)       
        }        
    }
    
    s := addProxyPassForColor("return", "protocol", "color", "upstreamname")
    fmt.Println(s)
    
    testDc := []string {"local:tag1,tag2,tag3","Demo:tag1,tag2,tag3","Castle:tag1,tag2,tag3"} 
    fmt.Printf("%v",parseDcPattern(testDc))
    
    w,b := getJoinDcLists()
    fmt.Printf("\nWhite\t%v",w)
    fmt.Printf("\nBlack\t%v",b)
    
    ok,err := filepath.Match("*l*","[local]")
    if ok{
        fmt.Println("Matched")
    } else {
        fmt.Println(err)
    }
    
    fmt.Println(cs)
    
}

func getCurrentPath() string{
    dir, err := os.Getwd()
    if err != nil {
        log.Fatal(err)
    }
    return dir  
}

func getHeaderTemplates(t1,t2 string) (r1,r2 []byte){
    dir := getCurrentPath()
    rt1,er1 := ioutil.ReadFile(path.Join(dir,t1))
    rt2,er2 := ioutil.ReadFile(path.Join(dir,t2))
    if er1 != nil{
        log.Fatal(er1)
    }
    if er2 != nil{
        log.Fatal(er2)
    }
    return rt1,rt2
}

func readServices() consulServices{
    raw,err := ioutil.ReadAll(os.Stdin)
	if err != nil{
		log.Fatal(err.Error())
	}
	
	var cs consulServices
	json.Unmarshal(raw,&cs)
	return cs
}

func addProxyPassForColor(locations, protocol, color, upstreamname string) string {
    
    s := `	if ($http_color = "%[2]s")  
	{ 
		proxy_pass %[1]s://%[2]s_%[3]s; break; 
	} 
	if ($cookie_color = "%[2]s") 
	{ 
		proxy_pass %[1]s://%[2]s_%[3]s; break; 
	}` 
    return locations + fmt.Sprintf(s, protocol,color,upstreamname)
}

func getJoinDcLists() ([]servicePattern, []servicePattern){
    wjc := strings.Split(os.Getenv("consul_joined_datacenters"),";")
    bjc := strings.Split(os.Getenv("consul_blocked_datacenters"),";")
    return parseDcPattern(wjc), parseDcPattern(bjc)
}

func parseDcPattern (strs [] string) []servicePattern {
    var svcPatterns []servicePattern
    for _,s := range strs{
       split := strings.Split(s,":")
       var sp servicePattern
       sp.DcPattern = split[0]
       if len(split) > 1 {
           sp.TagsPattern = strings.Split(split[1],",")
       }
       svcPatterns = append(svcPatterns,sp)       
    }
    return svcPatterns  
}

func isStringMatchingAnyPattern(str string, patterns []string) bool {
    for _,s := range patterns {
        ok,err := filepath.Match(s,str)
        if ok {
            return true
        }
        if err != nil {
            fmt.Println(err)
            return false
        }
    }
    return false
}

func isInstanceMatchingAnyPattern(ins instance, patterns []servicePattern) bool {
    for _,s := range patterns {
        ok,err := filepath.Match(s.DcPattern, ins.Dc)
        if ok {
            if len(s.TagsPattern) == 0 {
                return true
            }
            for _,t := range ins.Tags{       
                if isStringMatchingAnyPattern(t,s.TagsPattern) { 
                    return true
                }   
            }
        }
        if err != nil {
            fmt.Println(err)
            return false
        }
    }
    return false  
}

func getUpstreamServerDefinitions(ins []instance){
    sort.Sort(byString(ins))
    var svs []string
    for _,in := range ins{
        s := getUpstreamServerDefinition(in)
        svs = append(svs,s)
        
    }
}

func getUpstreamServerDefinition(ins instance) string {
    s := " down"
    if ins.Status == "passing" {
        s = ""
    }
    return fmt.Sprintf("\tserver %s:%s%s",ins.Address,ins.Port,s)
}

func hostnameResolves(host string) bool{
    _,err := net.LookupHost(host)
    return err == nil
}

