package util

import (
	"bytes"
	"context"
	"fmt"
	"hash/fnv"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

func Fnv1a32(key string) string {
	h := fnv.New32a()
	h.Write([]byte(key))
	hashed := h.Sum32()
	hashedStr := fmt.Sprintf("%x", hashed)
	return hashedStr
}

func ExecCommandInPod(config *rest.Config, podName, namespace, containerName, command string) error {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}
	cmd := []string{
		"sh",
		"-c",
		command,
	}
	const tty = false
	req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).SubResource("exec").Param("container", containerName)
	req.VersionedParams(
		&v1.PodExecOptions{
			Command: cmd,
			Stdin:   false,
			Stdout:  true,
			Stderr:  true,
			TTY:     tty,
		},
		scheme.ParameterCodec,
	)

	var stdout, stderr bytes.Buffer
	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return err
	}
	err = exec.StreamWithContext(context.TODO(), remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		if strings.TrimSpace(stderr.String()) != "" {
			return fmt.Errorf(strings.TrimSpace(stderr.String()))
		}
		return err
	}

	return nil
}

func ContainsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
