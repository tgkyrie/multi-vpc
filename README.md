# multi-vpc
在kube-ovn vpc-dns deployment上自动创建和维护Dns转发的operator

## Getting Started

### 环境要求
- go version v1.21.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### 构建镜像
**执行以下命令， `IMG`为构建的镜像名:**

```sh
export IMG=teriri152/multivpc:latest
make docker-build docker-push
```

### 生成yaml文件

```sh
make deploy
注：由于构建好deploy.yaml后需要加上一些ClusterRole，因此建议直接使用工程目录下的deploy.yaml
```

### 本地保存镜像文件，远程集群导入镜像文件

本地
```sh
sudo docker tag controller:latest teriri152/multivpc:latest

sudo docker save teriri152/multivpc:latest > myimage.tar
```

k8s集群
```sh
sudo ctr -n k8s.io image import myimage.tar
```

### k8s集群部署
```sh
kubectl apply -f deploy.yaml
```

### 测试

新建 dns.yaml
```yaml
apiVersion: "kubeovn.ustc.io/v1"
kind: VpcDnsForward
metadata:
  name: dns-1
  namespace: ns1
spec:
  vpc: test-vpc-1

```
```sh
kubectl apply -f dns.yaml
```
登陆vpc-dns deployment, 可以看到路由已经转发
```sh
kubectl delete -f dns.yaml
```
登陆vpc-dns deployment, 可以看到路由已被删除