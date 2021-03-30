// +build integration

package k8s

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"strconv"
	"testing"

	v1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	exitFail = 1

	envImageTag   = "VERSION"
	envInstallCNI = "INSTALL_CNI"
	envInstallCNS = "INSTALL_CNS"

	// relative azure-cni-manager path
	cniDaemonSetPath = "manifests/cni/manager.yaml"
	cniLabelSelector = "acn=azure-cni-manager"

	// relative cns manifest paths
	cnsManifestFolder         = "manifests/cns"
	cnsDaemonSetPath          = cnsManifestFolder + "/daemonset.yaml"
	cnsClusterRolePath        = cnsManifestFolder + "/clusterrole.yaml"
	cnsClusterRoleBindingPath = cnsManifestFolder + "/clusterrolebinding.yaml"
	cnsConfigMapPath          = cnsManifestFolder + "/configmap.yaml"
	cnsRolePath               = cnsManifestFolder + "/role.yaml"
	cnsRoleBindingPath        = cnsManifestFolder + "/rolebinding.yaml"
	cnsServiceAccountPath     = cnsManifestFolder + "/serviceaccount.yaml"
	cnsLabelSelector          = "k8s-app=azure-cns"
)

func TestMain(m *testing.M) {
	var (
		err        error
		exitCode   int
		clientset  *kubernetes.Clientset
		cnicleanup func() error
		cnscleanup func() error
	)

	defer func() {
		if r := recover(); r != nil {
			log.Println(string(debug.Stack()))
			exitCode = exitFail
		}

		if err != nil {
			log.Print(err)
			exitCode = exitFail
		} else {
			if cnicleanup != nil {
				cnicleanup()
			}
			if cnscleanup != nil {
				cnscleanup()
			}
		}

		os.Exit(exitCode)
	}()

	if clientset, err = mustGetClientset(); err != nil {
		return
	}

	ctx := context.Background()
	if installopt := os.Getenv(envInstallCNI); installopt != "" {
		// create dirty cni-manager ds
		if installCNI, err := strconv.ParseBool(installopt); err != nil && installCNI == true {
			if cnicleanup, err = installCNIManagerDaemonset(ctx, clientset, os.Getenv(envImageTag)); err != nil {
				log.Print(err)
				return
			}
		}
	} else {
		log.Printf("Env %v not set to true, skipping", envInstallCNI)
	}
	fmt.Printf("Before install cns")
	if installopt := os.Getenv(envInstallCNS); installopt != "" {
		fmt.Printf("In if CNS")
		// create dirty cns ds
		if installCNS, err := strconv.ParseBool(installopt); err != nil && installCNS == true {
			fmt.Printf("Thing was bool and true")
			if cnscleanup, err = installCNSDaemonset(ctx, clientset, os.Getenv(envImageTag)); err != nil {
				fmt.Printf("no error")
				return
			}
		}
	} else {
		fmt.Printf("Env %v not set to true, skipping", envInstallCNS)
		fmt.Printf("installopt %v", installopt)
		fmt.Printf("InstallCNS %v", installCNS)
		fmt.Printf("error %v", err)
		fmt.Printf("envImageTag %v", envImageTag)
		log.Printf("Env %v not set to true, skipping", envInstallCNS)
	}

	exitCode = m.Run()
}

func installCNSDaemonset(ctx context.Context, clientset *kubernetes.Clientset, imageTag string) (func() error, error) {
	var (
		err error
		cns v1.DaemonSet
	)

	if imageTag == "" {
		fmt.Print("No image")
		return nil, fmt.Errorf("Azure CNS image tag not set")
	}

	// setup daemonset
	if cns, err = mustParseDaemonSet(cnsDaemonSetPath); err != nil {
		fmt.Print("Must parse failed")
		return nil, err
	}

	image, _ := parseImageString(cns.Spec.Template.Spec.Containers[0].Image)
	cns.Spec.Template.Spec.Containers[0].Image = getImageString(image, imageTag)
	cnsDaemonsetClient := clientset.AppsV1().DaemonSets(cns.Namespace)

	log.Printf("Installing CNS with image %s", cns.Spec.Template.Spec.Containers[0].Image)
	fmt.Printf("Installing CNS")
	// setup the CNS configmap
	if err := mustSetupConfigMap(ctx, clientset, cnsConfigMapPath); err != nil {
		return nil, err
	}

	// setup common RBAC, ClusteerRole, ClusterRoleBinding, ServiceAccount
	if _, err := mustSetUpClusterRBAC(ctx, clientset, cnsClusterRolePath, cnsClusterRoleBindingPath, cnsServiceAccountPath); err != nil {
		return nil, err
	}

	// setup RBAC, Role, RoleBinding
	if err := mustSetUpRBAC(ctx, clientset, cnsRolePath, cnsRoleBindingPath); err != nil {
		return nil, err
	}

	if err = mustCreateDaemonset(ctx, cnsDaemonsetClient, cns); err != nil {
		return nil, err
	}

	if err = waitForPodsRunning(ctx, clientset, cns.Namespace, cnsLabelSelector); err != nil {
		return nil, err
	}

	cleanupds := func() error {
		if err := mustDeleteDaemonset(ctx, cnsDaemonsetClient, cns); err != nil {
			return err
		}
		return nil
	}

	return cleanupds, nil
}

func installCNIManagerDaemonset(ctx context.Context, clientset *kubernetes.Clientset, imageTag string) (func() error, error) {
	var (
		err error
		cni v1.DaemonSet
	)

	if cni, err = mustParseDaemonSet(cniDaemonSetPath); err != nil {
		return nil, err
	}

	// set the custom image tag and install
	image, _ := parseImageString(cni.Spec.Template.Spec.Containers[0].Image)
	cni.Spec.Template.Spec.Containers[0].Image = getImageString(image, imageTag)
	cniDaemonsetClient := clientset.AppsV1().DaemonSets(cni.Namespace)

	log.Printf("Installing CNI with  image %s", cni.Spec.Template.Spec.Containers[0].Image)

	if err = mustCreateDaemonset(ctx, cniDaemonsetClient, cni); err != nil {
		return nil, err
	}

	if err = waitForPodsRunning(ctx, clientset, cni.Namespace, cniLabelSelector); err != nil {
		return nil, err
	}

	cleanupds := func() error {
		if err := mustDeleteDaemonset(ctx, cniDaemonsetClient, cni); err != nil {
			return err
		}
		return nil
	}

	return cleanupds, nil
}
