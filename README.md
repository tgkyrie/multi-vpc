# multi-vpc
在kube-ovn vpc网关上自动创建和维护隧道的operator

## Getting Started

### 环境要求
- go version v1.21.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### 构建镜像
**执行以下命令， `IMG`为构建的镜像名:**

将makefile中的IMG改为镜像地址

```sh
make docker-build 
# 或make docker-build docker-push到远程仓库
```

### 生成yaml文件

更改了makefile中的deploy

```sh
make deploy
```

### 本地保存镜像文件，远程集群导入镜像文件

也可直接从远程仓库pull

本地
```sh
sudo docker save -o myimage.tar github.com/shenzuzhenwang/multi-vpc/multivpc:latest
```

k8s集群
```sh
sudo ctr -n k8s.io image import myimage.tar
```

### k8s集群部署

本地镜像，需要更改imagePullPolicy: IfNotPresent

```sh
kubectl apply -f deploy.yaml
```

### 测试

新建tunnel.yaml
```yaml
apiVersion: "kubeovn.ustc.io/v1"
kind: VpcNatTunnel
metadata:
  name: ovn-gre0
  namespace: ns2
spec:
  internalIp: "172.16.50.84" #vpc网关实体网络ip
  remoteIp: "172.16.50.122" #互联的对端vpc网关实体网络ip
  interfaceAddr: "10.0.0.1/24" #隧道地址
  natGwDp: "vpc2-net1-gateway" #vpc网关名字

  GlobalnetCIDR: "242.1.0.0/16"
  remoteGlobalnetCIDR: "242.0.0.0/16"

```
```sh
kubectl apply -f tunnel.yaml
```
登陆vpc网关pod，可以观察到隧道创建
```sh
kubectl delete -f tunnel.yaml
```
登陆vpc网关pod,可以观察到隧道被删除

## TODO
+ 处理VpcNatTunnel Update事件
+ Watch Vpc网关Pod的变更事件，维护隧道
+ ...