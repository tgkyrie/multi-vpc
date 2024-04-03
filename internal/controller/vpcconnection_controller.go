/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	testv1 "VpcConnection/api/v1"
	"bytes"
	"context"
	ovn "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/klog/v2"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
	"time"
)

// VpcConnectionReconciler reconciles a VpcConnection object
type VpcConnectionReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *rest.Config
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *VpcConnectionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	vc := testv1.VpcConnection{}
	err := r.Client.Get(ctx, req.NamespacedName, &vc)

	// 找到 VpcConnection 根据 Operation 字段来更新
	switch vc.Spec.Operation {
	// 建立 DNS 路由转发
	case testv1.DnsConnectionCreate:
		err := r.createDnsConnection(ctx, &vc)
		if err != nil {
			klog.Info("Connection start failed")
			return ctrl.Result{Requeue: true}, err
		}
	// 建立Vpc连接
	case testv1.VpcConnectionCreate:
		err := r.createConnection(ctx, &vc)
		if err != nil {
			klog.Info("Connection start failed")
			return ctrl.Result{Requeue: true}, err
		}
	// 恢复连接
	case testv1.VpcConnectionRecovery:
		err = r.recoveryConnection(ctx, &vc)
		if err != nil {
			klog.Info("Connection recovery failed")
			return ctrl.Result{Requeue: true}, err
		}
	// 删除连接
	case testv1.VpcConnectionStop:
		err = r.deleteConnection(ctx, &vc)
		if err != nil {
			klog.Info("Connection delete failed")
			return ctrl.Result{Requeue: true}, err
		}
	}
	return ctrl.Result{}, nil
}

// 建立 DNS 路由转发
func (r *VpcConnectionReconciler) createDnsConnection(ctx context.Context, vc *testv1.VpcConnection) error {
	state, err := r.checkDnsConnection(ctx, vc)
	if err != nil {
		return err
	}
	if state == true {
		r.updateOperation(ctx, vc, testv1.VpcConnectionCreate)
		return nil
	}
	vpcDnsDeployment, err := r.getVpcDnsDp(ctx, vc)
	if err != nil {
		return err
	}
	var coreDnsSvc corev1.Service
	err = r.Client.Get(ctx, client.ObjectKey{
		Name:      "kube-dns",
		Namespace: "kube-system",
	}, &coreDnsSvc)
	if err != nil {
		return err
	}
	var gateway ovn.VpcNatGateway
	var subnet ovn.Subnet
	err = r.Client.Get(ctx, client.ObjectKey{
		Namespace: "kube-system",
		Name:      vc.Spec.Gateway,
	}, &gateway)
	if err != nil {
		return err
	}
	// 获取对应的 SUBNET
	err = r.Client.Get(ctx, client.ObjectKey{
		Name: gateway.Spec.Subnet,
	}, &subnet)
	if err != nil {
		return err
	}
	initContainers := vpcDnsDeployment.Spec.Template.Spec.InitContainers
	for _, it := range initContainers {
		command := append(it.Command, " ip -4 route add "+coreDnsSvc.Spec.ClusterIP+" via "+subnet.Spec.Gateway+" dev net1;")
		it.Command = command
		klog.Info(command)
	}
	err = r.Client.Update(ctx, &vpcDnsDeployment)
	if err != nil {
		return err
	}
	return nil
}

// 获取 VPS-DNS DP
func (r *VpcConnectionReconciler) getVpcDnsDp(ctx context.Context, vc *testv1.VpcConnection) (appsv1.Deployment, error) {
	var vpcDnsList ovn.VpcDnsList
	var vpcDns ovn.VpcDns
	err := r.Client.List(ctx, &vpcDnsList, &client.ListOptions{})
	if err != nil {
		return appsv1.Deployment{}, err
	}
	// 寻找资源状态为 true 的 VPC-DNS
	for _, it := range vpcDnsList.Items {
		if it.Spec.Vpc == vc.Spec.Vpc && it.Status.Active == true {
			vpcDns = it
			break
		}
	}
	var vpcDnsDeployment appsv1.Deployment
	err = r.Client.Get(ctx, client.ObjectKey{
		Name:      "vpc-dns-" + vpcDns.Name,
		Namespace: "kube-system",
	}, &vpcDnsDeployment)
	if err != nil {
		return appsv1.Deployment{}, err
	}
	return vpcDnsDeployment, nil
}

// 获取 vpc-gateway pod ip
func (r *VpcConnectionReconciler) getGatewayIp(ctx context.Context, vc *testv1.VpcConnection) string {
	var gateway ovn.VpcNatGateway
	err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: "kube-system",
		Name:      vc.Spec.Gateway,
	}, &gateway)
	if err != nil {
		return ""
	}
	set := &appsv1.StatefulSet{}
	err = r.Client.Get(ctx, client.ObjectKey{
		Namespace: "kube-system",
		Name:      "vpc-nat-gw-" + vc.Spec.Gateway,
	}, set)
	if err != nil {
		return ""
	}
	podList := &corev1.PodList{}
	listOptions := client.ListOptions{
		LabelSelector: labels.Set(set.Spec.Selector.MatchLabels).AsSelector(),
	}
	err = r.Client.List(context.Background(), podList, &listOptions)
	var podIp string
	for _, pod := range podList.Items {
		podIp = pod.Status.PodIP
	}
	return podIp
}

// 检查 DNS 是否已经成功转发
func (r *VpcConnectionReconciler) checkDnsConnection(ctx context.Context, vc *testv1.VpcConnection) (bool, error) {
	var vpcDnsList ovn.VpcDnsList
	var vpcDns ovn.VpcDns
	var subnet ovn.Subnet
	err := r.Client.List(ctx, &vpcDnsList, &client.ListOptions{})
	if err != nil {
		return false, err
	}
	// 寻找资源状态为 true 的 VPC-DNS
	for _, it := range vpcDnsList.Items {
		if it.Spec.Vpc == vc.Spec.Vpc && it.Status.Active == true {
			vpcDns = it
			break
		}
	}
	// 获取对应的 SUBNET
	err = r.Client.Get(ctx, client.ObjectKey{
		Name: vpcDns.Spec.Subnet,
	}, &subnet)
	if err != nil {
		return false, err
	}
	if len(subnet.Spec.Namespaces) == 0 {
		return false, err
	}
	// CoreDNS SVC IP
	var coreDnsSvc corev1.Service
	err = r.Client.Get(ctx, client.ObjectKey{
		Name:      "kube-dns",
		Namespace: "kube-system",
	}, &coreDnsSvc)
	if err != nil {
		return false, err
	}
	// 构建临时 pod
	command := "dig no.ns1.svc.clusterset.local"
	nameSpace := subnet.Spec.Namespaces[0]
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dig-test",
			Namespace: nameSpace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "test",
					Image:   "nicolaka/netshoot",
					Command: []string{"/bin/bash", "-c", command},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}
	clientSet, err := kubernetes.NewForConfig(r.Config)
	if err != nil {
		return false, err
	}
	resultPod, err := clientSet.CoreV1().Pods(nameSpace).Create(context.TODO(), pod, metav1.CreateOptions{})
	if err != nil {
		panic(err.Error())
	}
	time.Sleep(10 * time.Second)
	podLog, err := clientSet.CoreV1().Pods(nameSpace).GetLogs(resultPod.Name, &corev1.PodLogOptions{}).Stream(context.TODO())
	if err != nil {
		return false, err
	}
	defer podLog.Close()
	err = clientSet.CoreV1().Pods(nameSpace).Delete(ctx, resultPod.Name, metav1.DeleteOptions{})
	if err != nil {
		return false, err
	}
	buf := new(bytes.Buffer)
	buf.ReadFrom(podLog)
	res := buf.String()
	if strings.Contains(res, coreDnsSvc.Spec.ClusterIP) {
		return true, nil
	} else {
		return false, nil
	}
}

// 建立 VPC 连接
func (r *VpcConnectionReconciler) createConnection(ctx context.Context, vc *testv1.VpcConnection) error {
	var gateway1 ovn.VpcNatGateway
	err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: "kube-system",
		Name:      vc.Spec.Gateway,
	}, &gateway1)
	if err != nil {
		return err
	}
	set := &appsv1.StatefulSet{}
	err = r.Client.Get(ctx, client.ObjectKey{
		Namespace: "kube-system",
		Name:      "vpc-nat-gw-" + vc.Spec.Gateway,
	}, set)
	if err != nil {
		return err
	}
	podList := &corev1.PodList{}
	listOptions := client.ListOptions{
		LabelSelector: labels.Set(set.Spec.Selector.MatchLabels).AsSelector(),
	}
	err = r.Client.List(context.Background(), podList, &listOptions)
	if err != nil {
		return err
	}
	for _, pod := range podList.Items {
		var content []byte
		content, err = os.ReadFile("./conf/create.sh")
		if err != nil {
			return err
		}
		script := string(content)
		_, err = r.execScript(pod.Name, pod.Namespace, script)
		if err != nil {
			return err
		}
	}
	r.updateStatus(ctx, vc, testv1.VpcConnectionRunning)
	return nil
}

// 更新 CR 的状态
func (r *VpcConnectionReconciler) updateStatus(ctx context.Context, vc *testv1.VpcConnection, state testv1.VpcConnectionState) {
	vc.Status.State = state
	err := r.Client.Update(ctx, vc)
	if err != nil {
		klog.Errorf("update status error:%v", err)
	}
}

// 更新 CR 的 Operation
func (r *VpcConnectionReconciler) updateOperation(ctx context.Context, vc *testv1.VpcConnection, operation testv1.VpcConnectionOperation) {
	vc.Spec.Operation = operation
	err := r.Client.Update(ctx, vc)
	if err != nil {
		klog.Errorf("update operation error:%v", err)
	}
}

// 恢复连接
func (r *VpcConnectionReconciler) recoveryConnection(ctx context.Context, vc *testv1.VpcConnection) error {
	return r.createConnection(ctx, vc)
}

// 断开连接
func (r *VpcConnectionReconciler) deleteConnection(ctx context.Context, vc *testv1.VpcConnection) error {
	var gateway1 ovn.VpcNatGateway
	err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: "kube-system",
		Name:      vc.Spec.Gateway,
	}, &gateway1)
	if err != nil {
		return err
	}
	set := &appsv1.StatefulSet{}
	err = r.Client.Get(ctx, client.ObjectKey{
		Namespace: "kube-system",
		Name:      "vpc-nat-gw-" + vc.Spec.Gateway,
	}, set)
	if err != nil {
		return err
	}
	podList := &corev1.PodList{}
	listOptions := client.ListOptions{
		LabelSelector: labels.Set(set.Spec.Selector.MatchLabels).AsSelector(),
	}
	err = r.Client.List(context.Background(), podList, &listOptions)
	if err != nil {
		return err
	}
	for _, pod := range podList.Items {
		var content []byte
		content, err = os.ReadFile("./conf/delete.sh")
		if err != nil {
			return err
		}
		script := string(content)
		_, err = r.execScript(pod.Name, pod.Namespace, script)
		if err != nil {
			return err
		}
	}
	err = r.Client.Delete(ctx, vc)
	if err != nil {
		return err
	}
	return nil
}

// 执行脚本文件
func (r *VpcConnectionReconciler) execScript(podName string, podNameSpace string, script string) (string, error) {
	clientSet, err := kubernetes.NewForConfig(r.Config)
	req := clientSet.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(podNameSpace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: []string{"/bin/bash", "-c", script},
			Stdin:   false,
			Stdout:  true,
			Stderr:  true,
			TTY:     false,
		}, scheme.ParameterCodec)
	executor, err := remotecommand.NewSPDYExecutor(r.Config, "POST", req.URL())
	if err != nil {
		return "", err
	}

	var stdout, stderr bytes.Buffer
	err = executor.Stream(remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    false,
	})
	if err != nil {
		return "", err
	}
	return stdout.String(), nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *VpcConnectionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{MaxConcurrentReconciles: 5}).
		For(&testv1.VpcConnection{}).
		Complete(r)
}
