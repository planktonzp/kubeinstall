package createcluster

import (
	"github.com/pkg/sftp"
	"kubeinstall/src/cluster"
	//"kubeinstall/src/config"
	"kubeinstall/src/cluster/dnsmonitor"
	"kubeinstall/src/cluster/heapster"
	"kubeinstall/src/cluster/influxdb"
	"kubeinstall/src/cluster/prometheus"
	"kubeinstall/src/cluster/redis"
	"kubeinstall/src/cmd"
	"kubeinstall/src/kubessh"
	"kubeinstall/src/logdebug"
	"kubeinstall/src/runconf"
	"kubeinstall/src/session"
	"time"
)

//测试下载
func testSftpDownload() {
	testSSH := kubessh.LoginInfo{
		HostAddr: "172.16.13.105",
		UserName: "root",
		Password: "docker",
		Port:     22,
	}

	client, _ := testSSH.ConnectToHost()

	sftpClient, _ := sftp.NewClient(client)

	kubessh.SftpDownload(sftpClient, "/home/a.txt", "/tmp")

	return
}

//测试sessionID
func testSessionID() {
	//session.Init()

	for {
		sessionID := session.Alloc()
		logdebug.Println(logdebug.LevelInfo, "-----申请sessionID----", sessionID, session.Get())
		session.Free(9)

		time.Sleep(1 * time.Second)

		if sessionID == 10 {
			return
		}
	}

	return
}

func testBuildCurrentStatus(testCluster cluster.Status) {
	cluster.SetStatus(testCluster)

	heapsterCMD := heapster.OptCMD{
		DockerCMDList: []cmd.ExecInfo{
			{CMDContent: "docker load --input $(DOCKER_IMAGES_PATH)/heapster.tar"},
			{CMDContent: "docker tag gcr.io/google_containers/jion/heapster:test-1.0.7 $(DOCKER_REGISTRY_URL)/jion/heapster:test-1.0.7"},
			{CMDContent: "docker push $(DOCKER_REGISTRY_URL)/jion/heapster:test-1.0.7"},
		},
		YamlCMDList: []cmd.ExecInfo{
			{CMDContent: "cp $(DOCKER_IMAGES_PATH)/heapster-src.yml $(DOCKER_IMAGES_PATH)/heapster.yaml"},
			{CMDContent: "sed -i \"s/gcr.io\\/google_containers/$(DOCKER_REGISTRY_URL)/g\" $(DOCKER_IMAGES_PATH)/heapster.yaml"},
			//{CMDContent: "sed -i \"s/cpu: 300m/cpu: $(CPU)m/g\" $(DOCKER_IMAGES_PATH)/heapster.yaml"},
			//{CMDContent: "sed -i \"s/memory: 512Mi/memory: $(MEM)Mi/g\" $(DOCKER_IMAGES_PATH)/heapster.yaml"},
			{CMDContent: "sed -i \"s/kubernetes.summary_api:port/$(K8S_API_URL)/g\" $(DOCKER_IMAGES_PATH)/heapster.yaml"},
		},
		CreateCMDList: []cmd.ExecInfo{
			{
				CMDContent: "kubectl create -f $(DEST_RECV_DIR)/heapster.yaml",
				EnvSet:     []string{"KUBECONFIG=/etc/kubernetes/admin.conf"},
			},
			{
				CMDContent: "kubectl create -f $(DEST_RECV_DIR)/heapster-rbac.yaml",
				EnvSet:     []string{"KUBECONFIG=/etc/kubernetes/admin.conf"},
			},
		},
	}
	runconf.Write(runconf.DataTypeHeapsterCMDConf, heapsterCMD)

	dnsMonitorCMD := dnsmonitor.OptCMD{
		DockerCMDList: []cmd.ExecInfo{
			{CMDContent: "docker load --input $(DOCKER_IMAGES_PATH)/busybox.tar"},
			{CMDContent: "docker tag gcr.io/google_containers/busybox:latest $(DOCKER_REGISTRY_URL)/busybox:latest"},
			{CMDContent: "docker push $(DOCKER_REGISTRY_URL)/busybox:latest"},
		},
	}
	runconf.Write(runconf.DataTypeDNSMonitorCMDConf, dnsMonitorCMD)

	influxdbCMD := influxdb.OptCMD{
		DockerCMDList: []cmd.ExecInfo{
			{CMDContent: "docker load --input $(DOCKER_IMAGES_PATH)/influxdb.tar"},
			{CMDContent: "docker tag gcr.io/google_containers/influxdb:0.7 $(DOCKER_REGISTRY_URL)/influxdb:0.7"},
			{CMDContent: "docker push $(DOCKER_REGISTRY_URL)/influxdb:0.7"},
		},
		YamlCMDList: []cmd.ExecInfo{
			{CMDContent: "cp $(DOCKER_IMAGES_PATH)/influxdb-src.yml $(DOCKER_IMAGES_PATH)/influxdb.yaml"},
			{CMDContent: "sed -i \"s/gcr.io\\/google_containers/$(DOCKER_REGISTRY_URL)/g\" $(DOCKER_IMAGES_PATH)/influxdb.yaml"},
			//{CMDContent: "sed -i \"s/cpu: 125m/cpu: $(CPU)m/g\" $(DOCKER_IMAGES_PATH)/influxdb.yaml"},
			//{CMDContent: "sed -i \"s/memory: 2048Mi/memory: $(MEM)Mi/g\" $(DOCKER_IMAGES_PATH)/influxdb.yaml"},
		},
		CreateCMDList: []cmd.ExecInfo{
			{
				CMDContent: "kubectl create -f $(DEST_RECV_DIR)/influxdb.yaml",
				EnvSet:     []string{"KUBECONFIG=/etc/kubernetes/admin.conf"},
			},
		},
	}
	runconf.Write(runconf.DataTypeInfluxdbCMDConf, influxdbCMD)

	prometheusCMD := prometheus.OptCMD{
		DockerCMDList: []cmd.ExecInfo{
			{CMDContent: "docker load --input $(DOCKER_IMAGES_PATH)/prometheus.tar"},
			{CMDContent: "docker tag gcr.io/google_containers/prometheus:v1027 $(DOCKER_REGISTRY_URL)/prometheus:v1027"},
			{CMDContent: "docker push $(DOCKER_REGISTRY_URL)/prometheus:v1027"},

			{CMDContent: "docker load --input $(DOCKER_IMAGES_PATH)/alertmanager.tar"},
			{CMDContent: "docker tag gcr.io/google_containers/alertmanager:v1027 $(DOCKER_REGISTRY_URL)/alertmanager:v1027"},
			{CMDContent: "docker push $(DOCKER_REGISTRY_URL)/alertmanager:v1027"},

			{CMDContent: "docker load --input $(DOCKER_IMAGES_PATH)/kube-state.tar"},
			{CMDContent: "docker tag gcr.io/google_containers/kube-state:m1 $(DOCKER_REGISTRY_URL)/kube-state:m1"},
			{CMDContent: "docker push $(DOCKER_REGISTRY_URL)/kube-state:m1"},

			{CMDContent: "docker load --input $(DOCKER_IMAGES_PATH)/node-exporter.tar"},
			{CMDContent: "docker tag gcr.io/google_containers/node-exporter:m1 $(DOCKER_REGISTRY_URL)/node-exporter:m1"},
			{CMDContent: "docker push $(DOCKER_REGISTRY_URL)/node-exporter:m1"},
		},
		YamlCMDList: []cmd.ExecInfo{
			{CMDContent: "cp $(DOCKER_IMAGES_PATH)/prometheus-deployment-src.yml $(DOCKER_IMAGES_PATH)/prometheus-deployment.yaml"},
			{CMDContent: "sed -i \"s/gcr.io\\/google_containers/$(DOCKER_REGISTRY_URL)/g\" $(DOCKER_IMAGES_PATH)/prometheus-deployment.yaml"},

			{CMDContent: "cp $(DOCKER_IMAGES_PATH)/alertmanager-deployment-src.yml $(DOCKER_IMAGES_PATH)/alertmanager-deployment.yaml"},
			{CMDContent: "sed -i \"s/gcr.io\\/google_containers/$(DOCKER_REGISTRY_URL)/g\" $(DOCKER_IMAGES_PATH)/alertmanager-deployment.yaml"},

			{CMDContent: "cp $(DOCKER_IMAGES_PATH)/kube-api-exporter-src.yml $(DOCKER_IMAGES_PATH)/kube-api-exporter.yaml"},
			{CMDContent: "sed -i \"s/gcr.io\\/google_containers/$(DOCKER_REGISTRY_URL)/g\" $(DOCKER_IMAGES_PATH)/kube-api-exporter.yaml"},

			{CMDContent: "cp $(DOCKER_IMAGES_PATH)/node-exporter-src.yml $(DOCKER_IMAGES_PATH)/node-exporter.yaml"},
			{CMDContent: "sed -i \"s/gcr.io\\/google_containers/$(DOCKER_REGISTRY_URL)/g\" $(DOCKER_IMAGES_PATH)/node-exporter.yaml"},

			//{CMDContent: "sed -i \"s/cpu: 125m/cpu: $(CPU)m/g\" $(DOCKER_IMAGES_PATH)/influxdb.yaml"},
			//{CMDContent: "sed -i \"s/memory: 2048Mi/memory: $(MEM)Mi/g\" $(DOCKER_IMAGES_PATH)/influxdb.yaml"},
		},
		CreateCMDList: []cmd.ExecInfo{
			{
				CMDContent: "kubectl create -f $(DEST_RECV_DIR)/prometheus-deployment.yaml",
				EnvSet:     []string{"KUBECONFIG=/etc/kubernetes/admin.conf"},
			},
			{
				CMDContent: "kubectl create -f $(DEST_RECV_DIR)/alertmanager-deployment.yaml",
				EnvSet:     []string{"KUBECONFIG=/etc/kubernetes/admin.conf"},
			},
			{
				CMDContent: "kubectl create -f $(DEST_RECV_DIR)/kube-api-exporter.yaml",
				EnvSet:     []string{"KUBECONFIG=/etc/kubernetes/admin.conf"},
			},
			{
				CMDContent: "kubectl create -f $(DEST_RECV_DIR)/node-exporter.yaml",
				EnvSet:     []string{"KUBECONFIG=/etc/kubernetes/admin.conf"},
			},
		},
	}
	runconf.Write(runconf.DataTypePrometheusCMDConf, prometheusCMD)

	redisCMD := redis.OptCMD{
		DockerCMDList: []cmd.ExecInfo{{CMDContent: "echo 'test redis docker cmd'"}},
		YamlCMDList:   []cmd.ExecInfo{{CMDContent: "echo 'test redis yaml cmd'"}},
		CreateCMDList: []cmd.ExecInfo{{CMDContent: "echo 'test redis create cmd'"}},
	}
	runconf.Write(runconf.DataTypeRedisCMDConf, redisCMD)

	return
}
