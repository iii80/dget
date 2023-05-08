# 用于直接从docker hub中下载镜像包

我们经常会遇到需要离线安装docker包的情况

如果每次都要安装docker，然后再去docker hub下载镜像包，这样的话，就会很麻烦，而且还会很慢

所以，我们可以直接使用dget从docker hub中下载镜像包，然后再离线安装

## 安装dget

```bash
go install gitee.com/extrame/dget/cmd/dget@latest
```

## 使用方法

```bash
dget influxdb:1.8.3
```

总之，就是dget后面跟docker镜像名，然后就会自动下载到当前目录的tmp_xxx目录下，下载有缓存支持，如果一次出错了，直接再次执行就可以了

成功的话，会直接生成tar.gz包

## 关于从第三方registry下载

```
dget alibaba-cloud-linux-3-registry.cn-hangzhou.cr.aliyuncs.com/alinux3/alinux3:220901.1
```

形如上述调用方法，直接在包名称前面跟上服务器地址即可（v1.0.1)

## 选择架构

最近很多的包都推出了多架构，命令增加了选择架构的功能

使用参数-arch可以指定下载的架构，例如 linux/arm等，请使用/分隔系统和架构，例如

```bash
dget -arch linux/arm influxdb:1.8.3
```

## 直接下载链接

[windows x64版本](https://dget.oss-cn-beijing.aliyuncs.com/dget_windows_amd64_v_1_0_1.zip)
[linux amd64版本](https://dget.oss-cn-beijing.aliyuncs.com/dget_linux_amd64_v_1_0_1.zip)
[linux arm版本](https://dget.oss-cn-beijing.aliyuncs.com/dget_linux_arm_v_1_0_1.zip)
[Mac 传统版本](https://dget.oss-cn-beijing.aliyuncs.com/dget_darwin_amd64_v1_0_1.zip)
[Mac arm64版本](https://dget.oss-cn-beijing.aliyuncs.com/dget_darwin_arm64_v1_0_1.zip)
