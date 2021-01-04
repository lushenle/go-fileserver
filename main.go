package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"html/template"
	"strconv"
	"strings"
)

const (
	maxUploadSize = 20 * 1024 * 1024 // 20 mb
	DefaultPort = 8000
)

var (
	port int
	rootPath string
)

var tpl = `
<!DOCTYPE html>
<html>
	<head>
		<title>fileserer - upload</title>
	</head>
	<body>
		<form enctype="multipart/form-data" action="http://{{.}}/upload" method="post">
			<input type="file" name="uploadFile" />
			<input type="submit" value="upload" />
		</form>
	</body>
</html>
`

func init() {
	flag.IntVar(&port, "port", DefaultPort, "The port to listen")
	flag.StringVar(&rootPath, "path", ".", "The path of the files")
	flag.Parse()
}

func renderError(w http.ResponseWriter, message string, statusCode int) {
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(message))
}

func dropError(err error) error {
	if err != nil {
		log.Fatal(err)
	}
	return err
}

func uploadFileHandler() http.HandlerFunc {
	localIps, err := GetLocalIPAddrs()
	dropError(err)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			t, err := template.New("upload").Parse(tpl)
			dropError(err)
			t.Execute(w, localIps[0]+":"+strconv.Itoa(port))
			return
		}
		// 解析、验证文件上传参数
		if err := r.ParseMultipartForm(maxUploadSize); err != nil {
			fmt.Printf("Could not parse multipart form: %v\n", err)
			renderError(w, "CANT_PARSE_FORM", http.StatusInternalServerError)
			return
		}
		file, fileHeader, err := r.FormFile("uploadFile")
		if err != nil {
			renderError(w, "INVALID_FILE", http.StatusBadRequest)
			return
		}

		defer file.Close()
		// 获取文件大小
		fileSize := fileHeader.Size
		// 判断文件大小是否大于上传限制
		if fileSize > maxUploadSize {
			renderError(w, "FILE_TOO_BIG", http.StatusBadRequest)
			return
		}
		fileBytes, err := ioutil.ReadAll(file)
		if err != nil {
			renderError(w, "INVALID_FILE", http.StatusBadRequest)
			return
		}

		fileName := fileHeader.Filename
		newPath := filepath.Join(rootPath, fileName)

		// 写文件
		newFile, err := os.Create(newPath)
		if err != nil {
			renderError(w, "CANT_WRITE_FILE", http.StatusInternalServerError)
			return
		}
		defer newFile.Close()
		if _, err := newFile.Write(fileBytes); err != nil || newFile.Close() != nil {
			renderError(w, "CANT_WRITE_FILE", http.StatusInternalServerError)
			return
		}
		w.Write([]byte(fileName + " upload success"))
	})
}

// GetLocalIPAddrs 获取本机IP
func GetLocalIPAddrs() ([]string, error) {
	addrs, err := net.InterfaceAddrs()
	dropError(err)

	ips := make([]string, 0)
	for _, address := range addrs {
		// 检查IP地址类型，获取非本地回环地址
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ips = append(ips, ipnet.IP.String())
			}
		}
	}
	return ips, nil
}

func main() {
	if port == 0 {
		port = DefaultPort
	}

	localIps, err := GetLocalIPAddrs()
	dropError(err)

	httpAddr := fmt.Sprintf(":%d", port)
	log.Printf("The root path is %s\n", rootPath)
	http.HandleFunc("/upload", uploadFileHandler())

	http.Handle("/", http.StripPrefix("/", http.FileServer(http.Dir(rootPath))))

	log.Printf("Service listen on port %d, and server ip addresses are %s" +
		", use /upload for uploading files and / for downloading\n",port, strings.Join(localIps, ", "))

	if err := http.ListenAndServe(httpAddr,nil); err != nil {
		log.Printf("http.ListendAndServer() failed with %s\n", err)
	}
	fmt.Printf("Exited\n")
}
