package main

import (
	"flag"
	"net/http"
	"net/url"
	"os"
	"strings"

	"gitee.com/extrame/dget"
	"github.com/sirupsen/logrus"
)

func main() {
	debug := flag.Bool("debug", false, "打印调试信息")
	printInfo := flag.Bool("print", false, "只打印获取信息")
	arch := flag.String("arch", "linux/amd64", "指定架构")
	proxy := flag.String("proxy", "", "http proxy")
	username := flag.String("username", "", "username")
	password := flag.String("password", "", "password")
	tags := flag.Bool("tags", false, "获取tag列表")
	var registry string
	flag.StringVar(&registry, "registry", "registry-1.docker.io", "指定镜像仓库")

	flag.Parse()

	if *debug {
		dget.SetLogLevel(logrus.DebugLevel)
	}

	args := flag.Args()
	logrus.Debugln("输入参数为", args)

	if len(args) == 0 {
		logrus.Fatalln("请输入需要下载的包名")
	}
	var pkg = args[0]
	var tag string
	if len(args) > 1 {
		tag = args[1]
	} else {
		var found bool
		pkg, tag, found = strings.Cut(pkg, ":")
		if !found {
			tag = "latest"
		}
		partsOfPkg := strings.Split(pkg, "/")
		if len(partsOfPkg) == 3 {
			registry = partsOfPkg[0]
			pkg = strings.Join(partsOfPkg[1:], "/")
		}
	}

	var client dget.Client
	if *proxy != "" {
		proxyUrl, err := url.Parse(*proxy)
		if err != nil {
			logrus.Fatalln("代理地址"+*proxy+"错误", err)
		}
		logrus.Info("use http proxy ", proxyUrl.String())
		client.SetClient(&http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(proxyUrl),
			},
		})
	} else {
		client.SetClient(http.DefaultClient)
	}

	var err = client.Install(registry, pkg, tag, *arch, *printInfo, *tags, *username, *password)
	if err != nil {
		logrus.Fatalln("下载发生错误", err)
	}
	os.Exit(0)
}
