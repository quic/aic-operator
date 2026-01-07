/*
Copyright (c) Qualcomm Technologies, Inc. and/or its subsidiaries.
SPDX-License-Identifier: BSD-3-Clause-Clear.
*/

package socreset

import (
	_ "embed"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	aicv1 "github.com/quic/aic-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
)

type SOCResetAPI interface {
	SetSOCResetDSasDesired(socresetdsObj *appsv1.DaemonSet, aic *aicv1.AIC) error
}

type socReset struct {
	client client.Client
	scheme *runtime.Scheme
}

func NewSOCReset(client client.Client, scheme *runtime.Scheme) SOCResetAPI {
	return &socReset{
		client: client,
		scheme: scheme,
	}
}

func (socreset_struct *socReset) SetSOCResetDSasDesired(dsObj *appsv1.DaemonSet, aic *aicv1.AIC) error {
	if dsObj == nil {
		return fmt.Errorf("daemon set is not initialized, zero pointer")
	}
	containerVolumeMounts := []v1.VolumeMount{
		{
			Name:      "socreset-state",
			MountPath: "/var/lib",
		},
	}
	hostPathDirectory := v1.HostPathDirectory
	volumes := []v1.Volume{
		{
			Name: "socreset-state",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/var/lib",
					Type: &hostPathDirectory,
				},
			},
		},
	}
	lifecyclehandler := v1.LifecycleHandler{
		Exec: &v1.ExecAction{
			Command: []string{"/bin/bash", "-c", "rm -f /var/lib/qaic_soc_reset_done"},
		},
	}
	lifecycle := v1.Lifecycle{
		PreStop: &lifecyclehandler,
	}

	matchLabels := map[string]string{"daemonset-name": aic.Name}
	//Below labels ensure that socreset Pod is created on nodes where qaic devices are present and boot-up is complete.
	devicepluginReady := fmt.Sprintf("kmm.node.kubernetes.io/%s.%s.device-plugin-ready", aic.Namespace, aic.Name)
	nodeSelector := map[string]string{"qualcomm.com/qaic": "true",
		devicepluginReady: ""}
	dsObj.Spec = appsv1.DaemonSetSpec{
		Selector: &metav1.LabelSelector{MatchLabels: matchLabels},
		Template: v1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: matchLabels,
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Command:         []string{"/bin/bash", "-c", "/opt/qti-aic/scripts/qaic-socreset.sh"},
						Name:            "qaic-socreset-container",
						Image:           aic.Spec.SocResetImage + ":" + aic.Spec.SocResetVersion,
						ImagePullPolicy: v1.PullAlways,
						SecurityContext: &v1.SecurityContext{Privileged: ptr.To(true)},
						VolumeMounts:    containerVolumeMounts,
						Lifecycle:       &lifecycle,
					},
				},
				NodeSelector:       nodeSelector,
				ServiceAccountName: "aic-operator-kmm-device-plugin",
				Volumes:            volumes,
			},
		},
	}

	return controllerutil.SetControllerReference(aic, dsObj, socreset_struct.scheme)
}
