package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
)

// Server is a data structure for NetScaler server data.
type Server struct {
	name      string
	ipAddress string
}

type Service struct {
	name     string
	server   Server
	protocol string
	port     string
	usip     string
}

// GetFile is a function that gets access to a file based on the file name.
func GetFile(fileName string) (string, error) {
	file, err := ioutil.ReadFile(fileName)
	if err != nil {
		return "", err
	}
	return string(file), nil
}

// GetConfig is a function that takes the contents of a file as a parameter as well as
// a pattern to use as a filter to return results as strings.
func GetConfig(file, pattern string) ([]string, error) {
	regexer, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	results := regexer.FindAllString(file, -1)
	return results, nil
}

// RemoveConfigKeywords is a function that removes the CLI keywords from within a NetScaler configuration.
func RemoveConfigKeywords(textLine, pattern string) string {
	result := strings.Replace(textLine, pattern, "", 1)
	return result
}

// QuoteIndex is a function that returns a 2D slice of integers.  This function helps to determine if there is a
// quote within a slice of strings.  If there is a quote this means that there is a space within a string and will
// have to be dealt with.  At the point in which this function is called, there may or may not be a quote in the first
// position of the string which is accepted as the parameter.
func QuoteIndex(line string) ([][]int, error) {
	pattern := "\"[a-zA-Z_0-9!<(/ ].*?[a-zA-Z_0-9)^\" /.']\""
	regexer, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	result := regexer.FindAllStringIndex(line, 1)
	return result, nil
}

// ExtractQuote is a function that uses a regular expression to extract strings that are surrounded by quotes.
// This function returns a string with the quotes.
func ExtractQuote(line string) (string, error) {
	pattern := "\"[a-zA-Z_0-9!<(/ ].*?[a-zA-Z_0-9)^\" /.']\""
	regexer, err := regexp.Compile(pattern)
	if err != nil {
		return "", err
	}
	result := regexer.FindString(line)
	return result, nil
}

// RemoveQuote is a function that removes quotes from a string.
func RemoveQuote(line string) string {
	pattern := "\""
	result := strings.Replace(line, pattern, "", -1)
	return result
}

// ExtractNoQuote is a function that extracts the first string followed by a space, that does not
// include a quote within the string.  While not intuitive, it works well with how the NetScaler config
// file is constructed.
func ExtractNoQuote(line string) (string, error) {
	pattern := "[A-Za-z0-9._].*?\\s"
	regexer, err := regexp.Compile(pattern)
	if err != nil {
		return "", err
	}
	result := regexer.FindString(line)
	return result, nil
}

// BuildServer is a function that accepts a file name as a parameter as well as server name as a string and returns a
// single Server type.
func BuildServer(fileName, serverName string) (Server, error) {
	file, err := GetFile(fileName)
	if err != nil {
		return Server{}, err
	}
	addServerLines, err := GetConfig(file, "(add server).*")
	if err != nil {
		return Server{}, err
	}
	for _, addServerLine := range addServerLines {
		serverLine := RemoveConfigKeywords(addServerLine, "add server ")
		quoteIndex, err := QuoteIndex(serverLine)
		if err != nil {
			return Server{}, err
		}
		length := len(quoteIndex)
		if length != 0 {
			intSlice := quoteIndex[0][0]
			if intSlice == 0 {
				extractedQuote, err := ExtractQuote(serverLine)
				if err != nil {
					return Server{}, err
				}
				lineTrim := strings.TrimSpace(extractedQuote)
				removedQuote := RemoveQuote(lineTrim)
				if serverName == removedQuote {
					var server Server
					server.name = removedQuote
					removeName := strings.Replace(serverLine, lineTrim, "", 1)
					trimSpace := strings.TrimSpace(removeName)
					server.ipAddress = trimSpace
					return server, nil
				}
			}
			if intSlice != 0 {
				// There are instances where comments are added to the server configuration.  This block handles situations
				// where comments are included.  Because comments are included, there are additional quotes surrounding
				// the comments.
				extractNoQuote, err := ExtractNoQuote(serverLine)
				if err != nil {
					return Server{}, err
				}
				lineTrim := strings.TrimSpace(extractNoQuote)
				if serverName == lineTrim {
					var server Server
					server.name = lineTrim
					removeName := strings.Replace(serverLine, extractNoQuote, "", 1)
					trimSpace := strings.TrimSpace(removeName)
					serverCommentLineArray := strings.Split(trimSpace, " ")
					server.ipAddress = serverCommentLineArray[0]
					return server, nil
				}
			}
		}
		if length == 0 {
			serverLineArray := strings.Split(serverLine, " ")
			if serverName == serverLineArray[0] {
				var server Server
				server.name = serverLineArray[0]
				server.ipAddress = strings.Replace(serverLineArray[1], "\r", "", -1)
				return server, nil
			}
		}
	}
	return Server{}, errors.New("no servers returned")
}

// GetServices is a function that returns an array of Load Balancing services.  It accepts a filename
// as a parameter.
func GetServices(fileName string) ([]Service, error) {
	file, err := GetFile(fileName)
	if err != nil {
		return []Service{}, err
	}
	addServiceLines, err := GetConfig(file, "(add service ).*")
	if err != nil {
		return []Service{}, err
	}
	var services []Service
	for _, addServiceLine := range addServiceLines {
		serviceLine := RemoveConfigKeywords(addServiceLine, "add service ")
		quoteIndex, err := QuoteIndex(serviceLine)
		if err != nil {
			return nil, err
		}
		length := len(quoteIndex)
		if length != 0 { // First quote for service name.
			intSlice := quoteIndex[0][0]
			if intSlice == 0 {
				extractedQuote, err := ExtractQuote(serviceLine)
				if err != nil {
					return nil, err
				}
				trimLine := strings.TrimSpace(extractedQuote)
				removedQuote := RemoveQuote(trimLine)
				var service Service
				service.name = removedQuote
				replaceName := strings.Replace(serviceLine, trimLine, "", 1)
				trimSpace := strings.TrimSpace(replaceName)
				quoteIndex, err := QuoteIndex(trimSpace)
				if err != nil {
					return nil, err
				}
				length := len(quoteIndex)
				if length != 0 { // Second quote for server name
					intSlice := quoteIndex[0][0]
					if intSlice == 0 {
						extractedQuote, err := ExtractQuote(trimSpace)
						if err != nil {
							return nil, err
						}
						trimLine := strings.TrimSpace(extractedQuote)
						removedQuote := RemoveQuote(trimLine)
						serviceServer, err := BuildServer(fileName, removedQuote)
						if err != nil {
							return nil, err
						}
						service.server = serviceServer
						replaceName := strings.Replace(trimSpace, extractedQuote, "", 1)
						trimSpace := strings.TrimSpace(replaceName)
						serviceLineArray := strings.Split(trimSpace, " ")
						service.protocol = serviceLineArray[0]
						service.port = serviceLineArray[1]
						for ix, arr := range serviceLineArray {
							if arr == "-usip" {
								service.usip = serviceLineArray[ix+1]
							}
						}
						services = append(services, service)
					}
					if intSlice != 0 {
						// As of now, this situation does not need to be handled.  There should not be any other
						// quotes.  If that changes, the code would go here.
					}
				}
				if length == 0 { // No quote for server name
					serviceLineArray := strings.Split(trimSpace, " ")
					service.server, err = BuildServer(fileName, serviceLineArray[0])
					if err != nil {
						return nil, err
					}
					service.protocol = serviceLineArray[1]
					service.port = serviceLineArray[2]
					for ix, arr := range serviceLineArray {
						if arr == "-usip" {
							service.usip = serviceLineArray[ix+1]
						}
					}
					services = append(services, service)
				}
			}
			if intSlice != 0 { // No quote for service name.
				extractNoQuote, err := ExtractNoQuote(serviceLine)
				if err != nil {
					return nil, err
				}
				trimNoQuote := strings.TrimSpace(extractNoQuote)
				var service Service
				service.name = trimNoQuote
				replaceNoQuote := strings.Replace(serviceLine, extractNoQuote, "", 1)
				trimReplace := strings.TrimSpace(replaceNoQuote)
				quoteIndex, err := QuoteIndex(trimReplace)
				if err != nil {
					return nil, err
				}
				length := len(quoteIndex)
				if length != 0 { // Quote for server name of service name with no quote.
					intSlice := quoteIndex[0][0]
					if intSlice == 0 {
						extractQuote, err := ExtractQuote(trimReplace)
						if err != nil {
							return nil, err
						}
						trimQuote := strings.TrimSpace(extractQuote)
						removeQuote := RemoveQuote(trimQuote)
						service.server, err = BuildServer(fileName, removeQuote)
						if err != nil {
							return nil, err
						}
						replaceQuote := strings.Replace(trimReplace, extractQuote, "", 1)
						trimQuote = strings.TrimSpace(replaceQuote)
						serviceLineArray := strings.Split(trimQuote, " ")
						service.protocol = serviceLineArray[0]
						service.port = serviceLineArray[1]
						for ix, arr := range serviceLineArray {
							if arr == "-usip" {
								service.usip = serviceLineArray[ix+1]
							}
						}
					}
				}
				services = append(services, service)
			}
		}
		if length == 0 {
			// This section is for no quotes detected.
			trimSpace := strings.TrimSpace(serviceLine)
			serviceLineArray := strings.Split(trimSpace, " ")
			var service Service
			service.name = serviceLineArray[0]
			service.server, err = BuildServer(fileName, serviceLineArray[1])
			if err != nil {
				return nil, err
			}
			service.protocol = serviceLineArray[2]
			service.port = serviceLineArray[3]
			for ix, arr := range serviceLineArray {
				if arr == "-usip" {
					service.usip = serviceLineArray[ix+1]
				}
			}
			services = append(services, service)
		}
	}
	return services, nil
}

// CreateFile is a function that accepts a file name as a parameter and returns a pointer to a file.
func CreateFile(fileName string) (*os.File, error) {
	file, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return file, nil
}

// main contains the business logic of the program.  It returns a file with the Load Balancing service name, server
// name and server IP address of services that are using usip (use source IP address).
func main() {
	filename := os.Args[1]
	services, err := GetServices(filename)
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, service := range services {
		if service.usip == "YES" {
			file, err := CreateFile(filename + "-usip-output.txt")
			if err != nil {
				fmt.Println(err)
			}
			fmt.Fprintln(file, service.name+" "+service.server.name+" "+service.server.ipAddress)
		}
	}
}
