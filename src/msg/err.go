package msg

//ErrorCode 错误码
type ErrorCode int

//Success 除了Success其他均为失败 其他 如非高可用这类错误 前端可以适当放宽
const (
	Success ErrorCode = iota //成功

	SSHNotAvailable //SSH不可用

	MasterNotHighAvailable  //master非高可用
	MasterNotSpecified      //master未指定
	HAVirtualIPNotSpecified //HA虚拟IP未指定

	EtcdNotHighAvailable //etcd非高可用
	EtcdNotSpecified     //etcd未指定
	EtcdCountInvalid     //etcd个数非法(建议为奇数个)

	YUMRegistryNotSpecified //yum仓库未指定
	YUMUserParamInvalid     //yum用户参数不合法

	DockerRegistryNotSpecified     //docker仓库未指定
	DockerRegistryPathNotSpecified //Docker仓库目录未指定   //2018.5.16
	DockerRegistryIPConflict       //Docker仓库IP与masterIP冲突
	DockerStorageParamInvalid      //docker存储参数不合法

	//NetworkCNINotSpecified //网络CNI没有指定

	CephRolesSetError      //ceph角色设定错误
	CephHostnameSetInvalid //ceph主机名设置不合法

	EntrySSHNotSpecified //entrySSH信息未指定

	ErrCodeMax
)

//GetErrMsg 获取错误码
func GetErrMsg(errCode ErrorCode) string {
	if errCode >= ErrCodeMax {
		return "非法的错误码"
	}

	errSet := []string{
		Success:                    "",
		SSHNotAvailable:            "SSH 不可用!\n",
		MasterNotSpecified:         "没有指定master节点!\n",
		MasterNotHighAvailable:     "master不是高可用方案!\n",
		HAVirtualIPNotSpecified:    "HA虚拟IP未指定!\n",
		EtcdNotHighAvailable:       "etcd不是高可用方案!\n",
		EtcdNotSpecified:           "etcd未指定!\n",
		EtcdCountInvalid:           "etcd节点个数不合法(建议奇数个)!\n",
		YUMUserParamInvalid:        "yum仓库用户配置非法!\n",
		YUMRegistryNotSpecified:    "yum仓库未指定!\n",
		DockerRegistryNotSpecified: "docker仓库未指定!\n",
		DockerRegistryIPConflict:   "docker仓库IP与master节点冲突!\n",
		//NetworkCNINotSpecified:     "网络CNI没有指定!\n",
		CephRolesSetError:         "ceph角色设定错误!\n",
		CephHostnameSetInvalid:    "ceph主机名设置不合法!\n",
		EntrySSHNotSpecified:      "entrySSH信息未指定!\n",
		DockerStorageParamInvalid: "docker存储参数不合法!\n",
	}

	return errSet[errCode]
}
