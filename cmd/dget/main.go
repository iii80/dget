package main

import (
	"flag"
	"os"
	"strings"

	"gitee.com/extrame/dget"
	"github.com/sirupsen/logrus"
)

func main() {
	debug := flag.Bool("debug", false, "打印调试信息")
	printInfo := flag.Bool("print", false, "只打印获取信息")
	arch := flag.String("arch", "linux/amd64", "指定架构")

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
	}
	err := dget.Install(pkg, tag, *arch, *printInfo)
	if err != nil {
		logrus.Fatalln("下载发生错误", err)
	}
	os.Exit(0)
}
