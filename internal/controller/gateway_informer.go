package controller

import (
	"context"
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	kubeovnv1 "multi-vpc/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

type GatewayInformer struct {
	Client client.Client
	Config *rest.Config
}

func New(client client.Client, config *rest.Config) *GatewayInformer {
	return &GatewayInformer{Client: client, Config: config}
}

func (r *GatewayInformer) Start(ctx context.Context) error {
	clientSet, err := kubernetes.NewForConfig(r.Config)
	var vpcNatTunnelList kubeovnv1.VpcNatTunnelList
	if err != nil {
		return err
	}
	labelSelector := labels.Set{
		"ovn.kubernetes.io/vpc-nat-gw": "true",
	}
	informer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				options.LabelSelector = labelSelector.AsSelector().String()
				return clientSet.AppsV1().StatefulSets("kube-system").List(ctx, options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				options.LabelSelector = labelSelector.AsSelector().String()
				return clientSet.AppsV1().StatefulSets("kube-system").Watch(ctx, options)
			},
		},
		&appsv1.StatefulSet{},
		0,
		cache.Indexers{},
	)
	_, err = informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		// add 方法对应 创建 Vpc-Gateway Statefulset 时执行的操作，感觉用不上
		AddFunc: func(obj interface{}) {
			statefulSet := obj.(*appsv1.StatefulSet)
			// 通过 Vpc-Gateway 的名称找到对应的VpcNatTunnel，可能有多个VpcNatTunnel，因此获取VpcNatTunnelList
			natGw := strings.TrimPrefix(statefulSet.Name, "vpc-nat-gw-")
			labelsSet := map[string]string{
				"NatGwDp": natGw,
			}
			option := client.ListOptions{
				LabelSelector: labels.SelectorFromSet(labelsSet),
			}
			err = r.Client.List(ctx, &vpcNatTunnelList, &option)
			if err != nil {
				return
			}
			if statefulSet.Status.AvailableReplicas == 1 {
				for _, vpcNatTunnel := range vpcNatTunnelList.Items {
					// 更新 vpcNatTunnel 状态
				}
			}
		},
		// update 方法对应 Vpc-Gateway Statefulset 状态更新时执行的操作
		UpdateFunc: func(old, new interface{}) {
			oldStatefulSet := old.(*appsv1.StatefulSet)
			newStatefulSet := new.(*appsv1.StatefulSet)
			// 通过 Vpc-Gateway 的名称找到对应的VpcNatTunnel，可能有多个VpcNatTunnel，因此获取VpcNatTunnelList
			natGw := strings.TrimPrefix(newStatefulSet.Name, "vpc-nat-gw-")
			labelsSet := map[string]string{
				"NatGwDp": natGw,
			}
			option := client.ListOptions{
				LabelSelector: labels.SelectorFromSet(labelsSet),
			}
			err = r.Client.List(ctx, &vpcNatTunnelList, &option)
			if err != nil {
				return
			}
			// Vpc-Gateway 节点重启，可用 pod 从 0 到 1
			if oldStatefulSet.Status.AvailableReplicas == 0 && newStatefulSet.Status.AvailableReplicas == 1 {
				for _, vpcNatTunnel := range vpcNatTunnelList.Items {
					// 更新 vpcNatTunnel 状态
				}
			}
			// Vpc-Gateway 节点宕掉， 可用 pod 从 1 到 0
			if oldStatefulSet.Status.AvailableReplicas == 1 && newStatefulSet.Status.AvailableReplicas == 0 {
				for _, vpcNatTunnel := range vpcNatTunnelList.Items {
					// 更新 vpcNatTunnel 状态
				}
			}
		},
		// delete 方法对应删除 Vpc-Gateway Statefulset 时执行的操作，感觉也用不上
		DeleteFunc: func(obj interface{}) {
			statefulSet := obj.(*appsv1.StatefulSet)
			// 通过 Vpc-Gateway 的名称找到对应的VpcNatTunnel，可能有多个VpcNatTunnel，因此获取VpcNatTunnelList
			natGw := strings.TrimPrefix(statefulSet.Name, "vpc-nat-gw-")
			labelsSet := map[string]string{
				"NatGwDp": natGw,
			}
			option := client.ListOptions{
				LabelSelector: labels.SelectorFromSet(labelsSet),
			}
			err = r.Client.List(ctx, &vpcNatTunnelList, &option)
			if err != nil {
				return
			}
			for _, vpcNatTunnel := range vpcNatTunnelList.Items {
				// 更新 vpcNatTunnel 状态
			}
		},
	})
	if err != nil {
		return err
	}
	stopCh := make(chan struct{})
	defer close(stopCh)
	go informer.Run(stopCh)
	if !cache.WaitForCacheSync(stopCh, informer.HasSynced) {
		return fmt.Errorf("Error syncing cache\n")
	}
	select {
	case <-stopCh:
		klog.Info("Received termination signal, exiting")
	}
	return nil
}
