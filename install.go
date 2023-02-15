package dget

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
)

const _registry = "registry-1.docker.io"
const _authUrl = "https://auth.docker.io/token"
const _regService = "registry.docker.io"

type LayerInfo struct {
	Id              string    `json:"id"`
	Parent          string    `json:"parent"`
	Created         time.Time `json:"created"`
	ContainerConfig struct {
		Hostname     string
		Domainname   string
		User         string
		AttachStdin  bool
		AttachStdout bool
		AttachStderr bool
		Tty          bool
		OpenStdin    bool
		StdinOnce    bool
		Env          []string
		CMd          []string
		Image        string
		Volumes      map[string]interface{}
		WorkingDir   string
		Entrypoint   []string
		OnBuild      []string
		Labels       map[string]interface{}
	} `json:"container_config"`
}

type Layer struct {
	Digest string
	Urls   []string
}

type Info struct {
	Layers []Layer `json:"layers"`
	Config struct {
		Digest digest.Digest `json:"digest,omitempty"`
	} `json:"config"`
}

type PackageConfig struct {
	Config   string
	RepoTags []string
	Layers   []string
}

func Install(d, tag string, arch string, printInfo bool) (err error) {
	var authUrl = _authUrl
	var regService = _regService
	resp, err := http.Get(fmt.Sprintf("https://%s/v2/", _registry))
	if err == nil {
		if !strings.Contains(d, "/") {
			d = "library/" + d
		}
		if resp.StatusCode == 401 {
			//Bearer realm="https://auth.docker.io/token",service="registry.docker.io"
			var hAuths = strings.Split(resp.Header.Get("Www-Authenticate"), "\"")
			if len(hAuths) > 1 {
				authUrl = hAuths[1]
			}
			if len(hAuths) > 3 {
				regService = hAuths[3]
			} else {
				regService = ""
			}
		}
		resp.Body.Close()
		var accessToken string
		logrus.Debugln("reg_service", regService)

		accessToken, err = getAuthHead("application/vnd.docker.distribution.manifest.v2+json", authUrl, regService, d)
		if err == nil {

			var req *http.Request

			var url = fmt.Sprintf("https://%s/v2/%s/manifests/%s", _registry, d, tag)
			req, err = http.NewRequest("GET", url, nil)
			logrus.Infoln("获取manifests信息", url)
			if err == nil {
				logrus.Debugln("Authorization by", accessToken)
				req.Header.Add("Authorization", "Bearer "+accessToken)
				req.Header.Add("Accept", "application/vnd.docker.distribution.manifest.list.v2+json")

				var authHeader = req.Header

				resp, err = http.DefaultClient.Do(req)
				if resp.StatusCode != 200 {
					bts, er := ioutil.ReadAll(resp.Body)
					resp.Body.Close()
					logrus.Debugln(string(bts), er)
					switch resp.StatusCode {
					case 401:
						logrus.Errorf("[-] Cannot fetch manifest for %s [HTTP %d] with error access_token", d, resp.StatusCode)
					case 404:
						logrus.Errorf("[-] Cannot fetch manifest for %s [HTTP %d] with url %s", d, resp.StatusCode, url)
						resp.Body.Close()
						req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.list.v2+json")
						resp, err = http.DefaultClient.Do(req)
						bts, er := ioutil.ReadAll(resp.Body)
						fmt.Println(string(bts), er)
					}
					//TODO

					os.Exit(1)
				} else {
					var info manifestlist.ManifestList
					var bts []byte
					bts, err = io.ReadAll(resp.Body)

					if err == nil {

						err = json.Unmarshal(bts, &info)

						if err == nil {
							resp.Body.Close()

							logrus.Infof("获得%d个架构信息:", len(info.Manifests))

							var selectedManifest *manifestlist.ManifestDescriptor
							for i := 0; i < len(info.Manifests); i++ {
								var m = info.Manifests[i]
								logrus.Infof("[%d]架构:%s,OS:%s", i+1, m.Platform.Architecture, m.Platform.OS)
								if m.Platform.OS+"/"+m.Platform.Architecture == arch {
									logrus.Infoln("找到匹配的架构,开始下载")
									selectedManifest = &m
								}
							}
							if printInfo {
								fmt.Println(string(bts))
								os.Exit(0)
							}

							if selectedManifest == nil {
								return errors.New("未找到匹配的架构:" + arch)
							}

							req.Header.Set("Accept", selectedManifest.MediaType)

							resp, err = http.DefaultClient.Do(req)
							var info Info
							err = json.NewDecoder(resp.Body).Decode(&info)

							if err == nil {
								resp.Body.Close()
								logrus.Infof("获得Manifest信息，共%d层需要下载", len(info.Layers))

								var tmpDir = fmt.Sprintf("tmp_%s_%s", d, tag)
								err = os.MkdirAll(tmpDir, 0777)
								if err == nil {
									if _, e := os.Stat(filepath.Join(tmpDir, "repositories")); e == nil {
										logrus.Info(tmpDir, "is downloaded,use dir as cache")
									} else {
										req, err = http.NewRequest("GET", fmt.Sprintf("https://%s/v2/%s/blobs/%s", _registry, d, info.Config.Digest), nil)
										if err == nil {
											req.Header = authHeader
											resp, err = http.DefaultClient.Do(req)
											if err == nil {
												var dest *os.File
												dest, err = os.OpenFile(filepath.Join(tmpDir, info.Config.Digest.Encoded()+".json"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
												if err == nil {
													var bts []byte
													bts, err = ioutil.ReadAll(resp.Body)
													var lastLayerInfo LayerInfo
													err = json.Unmarshal(bts, &lastLayerInfo)
													resp.Body.Close()

													var config []PackageConfig
													config = append(config, PackageConfig{
														Config:   info.Config.Digest.Encoded() + ".json",
														RepoTags: []string{d + ":" + tag},
													})
													if err == nil {
														_, err = io.Copy(dest, bytes.NewReader(bts))
														dest.Close()
														if err == nil {
															parentid := ""
															var fakeLayerId string
															for n, layer := range info.Layers {
																namer := sha256.New()
																namer.Write([]byte(parentid + "\n" + layer.Digest + "\n"))
																fakeLayerId = hex.EncodeToString(namer.Sum(nil))
																logrus.Infoln("handle layer", n, fakeLayerId, layer.Urls)
																layerDirName := filepath.Join(tmpDir, fakeLayerId)
																err = os.Mkdir(layerDirName, 0777)
																if _, er := os.Stat(filepath.Join(layerDirName, "layer.tar")); er == nil {
																	logrus.Infoln("layer", fakeLayerId, "is existed, continue")
																	config[0].Layers = append(config[0].Layers, fakeLayerId+"/layer.tar")
																	parentid = fakeLayerId
																	continue
																}
																if err == nil || os.IsExist(err) {
																	err = ioutil.WriteFile(filepath.Join(layerDirName, "VERSION"), []byte("1.0"), 0666)
																	if err == nil {
																		req, err = http.NewRequest("GET", fmt.Sprintf("https://%s/v2/%s/blobs/%s", _registry, d, layer.Digest), nil)
																		if err == nil {
																			req.Header = authHeader
																			req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")
																			resp, err = http.DefaultClient.Do(req)
																			if err == nil {
																				if resp.StatusCode != 200 {
																					defer resp.Body.Close()
																					if len(layer.Urls) > 0 {
																						req, err = http.NewRequest("GET", layer.Urls[0], nil)
																						if err == nil {
																							req.Header = authHeader
																							req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")
																							resp, err = http.DefaultClient.Do(req)
																							if err == nil {
																								if resp.StatusCode != 200 {
																									err = fmt.Errorf("download from customized url fail for layer[%d]", n)
																									goto response
																								}
																							}
																						}
																					} else {
																						bts, _ := ioutil.ReadAll(resp.Body)
																						logrus.Fatalln("下载失败", string(bts))
																					}
																				}
																			}
																			if err != nil {
																				logrus.Errorf("请求第%d/%d层失败:%v", n+1, len(info.Layers), err)
																			} else {
																				logrus.Infof("请求第%d/%d层成功", n+1, len(info.Layers))
																			}
																			var dst *os.File
																			dst, err = os.OpenFile(filepath.Join(layerDirName, "layer.tar.part"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
																			if err == nil {
																				var greader *gzip.Reader
																				greader, err = gzip.NewReader(resp.Body)
																				if err == nil {
																					_, err = io.Copy(dst, greader)
																					if err == nil {
																						dst.Close()
																						var layerInfo LayerInfo
																						if n == len(info.Layers)-1 {
																							layerInfo = lastLayerInfo
																						}
																						layerInfo.Id = fakeLayerId
																						if parentid != "" {
																							layerInfo.Parent = parentid
																						}
																						parentid = fakeLayerId
																						var jsonFile *os.File
																						jsonFile, err = os.OpenFile(filepath.Join(layerDirName, "json"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
																						if err == nil {
																							err = json.NewEncoder(jsonFile).Encode(&layerInfo)
																							if err == nil {
																								jsonFile.Close()
																								err = os.Rename(filepath.Join(layerDirName, "layer.tar.part"), filepath.Join(layerDirName, "layer.tar"))
																							}
																						}
																					}
																				}
																			}
																			if err != nil {
																				logrus.Errorf("保存第%d/%d层失败,%v", n+1, len(info.Layers), err)
																			} else {
																				logrus.Infof("保存第%d/%d层成功", n+1, len(info.Layers))
																			}
																			if err != nil {
																				goto response
																			} else {
																				config[0].Layers = append(config[0].Layers, fakeLayerId+"/layer.tar")
																			}
																		}
																	}
																}
															}
															var manifest *os.File
															manifest, err = os.OpenFile(filepath.Join(tmpDir, "manifest.json"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
															if err == nil {
																err = json.NewEncoder(manifest).Encode(&config)
																if err == nil {
																	manifest.Close()
																	var repositories = make(map[string]interface{})
																	repositories[d] = map[string]string{
																		tag: fakeLayerId,
																	}
																	var rFile *os.File
																	rFile, err = os.OpenFile(filepath.Join(tmpDir, "repositories"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
																	if err == nil {
																		err = json.NewEncoder(rFile).Encode(&repositories)
																		goto maketar
																	}
																}

															}
														}
													}
												}
											}
										}

									}
								maketar:
									if err == nil {
										err = writeDirToTarGz(tmpDir, tmpDir+"-img.tar.gz")
										if err == nil {
											fmt.Println("write tar success", tmpDir+"-img.tar.gz")
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
response:
	return
}

func getAuthHead(u, a, r, d string) (string, error) {
	resp, err := http.Get(fmt.Sprintf("%s?service=%s&scope=repository:%s:pull", a, r, d))
	defer resp.Body.Close()
	if err == nil {
		var results map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&results)
		logrus.Debug(results)
		if err == nil {
			return results["access_token"].(string), nil
		}
	}
	return "", err
}

func writeDirToTarGz(sourcedir, destinationfile string) error {
	// create tar file
	gzFile, err := os.Create(destinationfile)
	gf := gzip.NewWriter(gzFile)
	tw := tar.NewWriter(gf)
	if err == nil {

		defer func() {
			tw.Close()
			gf.Close()
			gzFile.Close()
		}()

		// get list of files
		return filepath.Walk(sourcedir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			relPath, err := filepath.Rel(sourcedir, path)
			if err == nil && relPath != "." {
				header, err := tar.FileInfoHeader(info, path)
				if err != nil {
					return err
				}

				// must provide real name
				// (see https://golang.org/src/archive/tar/common.go?#L626)
				header.Name = filepath.ToSlash(relPath)

				// write header
				if err := tw.WriteHeader(header); err != nil {
					return err
				}
				// if not a dir, write file content
				if !info.IsDir() {
					data, err := os.Open(path)
					if err != nil {
						return err
					}
					if _, err := io.Copy(tw, data); err != nil {
						return err
					}
				}
				return nil
			}
			return err
		})

	}
	return err
}

func SetLogLevel(lvl logrus.Level) {
	logrus.SetLevel(lvl)
	logrus.Debugln("设置日志级别为", lvl)
}
