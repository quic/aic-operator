/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

Based on code from https://github.com/yevgeny-shnaidman/amd-gpu-operator

Copyright (c) 2024-2025 Qualcomm Innovation Center, Inc. All rights reserved.
SPDX-License-Identifier: BSD-3-Clause-Clear
Not a contribution.
*/

package controller

import (
	"context"
	"errors"
	"fmt"

	kmmv1beta1 "github.com/rh-ecosystem-edge/kernel-module-management/api/v1beta1"
	nfr "github.com/openshift/cluster-nfd-operator/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	aicv1 "github.com/quic/aic-operator/api/v1"
	"github.com/quic/aic-operator/internal/kmmmodule"
	"github.com/quic/aic-operator/internal/nfdrule"
)

// AICReconciler reconciles an AIC object
type AICReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	helper AICReconcilerHelperAPI
}

//+kubebuilder:rbac:groups=aic.quicinc.com,resources=aics,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=aic.quicinc.com,resources=aics/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=aic.quicinc.com,resources=aics/finalizers,verbs=update
//+kubebuilder:rbac:groups=kmm.sigs.x-k8s.io,resources=modules,verbs=get;list;watch;create;patch;update;delete
//+kubebuilder:rbac:groups=kmm.sigs.x-k8s.io,resources=modules/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=nfd.openshift.io,resources=nodefeaturerules,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=nfd.openshift.io,resources=nodefeaturerules/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;clusterrolebindings;roles;rolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=namespaces;serviceaccounts;pods;pods/exec;pods/attach;services;services/finalizers;endpoints,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=configmaps;secrets;nodes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments;daemonsets;replicasets;statefulsets,verbs=get;list;watch;create;update;patch;delete


func NewAICReconciler(
	client client.Client,
	scheme *runtime.Scheme,
	kmmHandler kmmmodule.KMMModuleAPI,
        nfdHandler nfdrule.NFDRuleAPI) *AICReconciler {
	helper := newAICReconcilerHelper(client, kmmHandler, nfdHandler)
	return &AICReconciler{
		Client: client,
		Scheme: scheme,
		helper: helper,
	}
}

const (
	aicFinalizer = "aic-operator.node.kubernetes.io/finalizer"
)

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the AIC object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *AICReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	res := ctrl.Result{}
	logger := log.FromContext(ctx)

	aic, err := r.helper.getRequestedAIC(ctx, req.NamespacedName)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			logger.Info("Module deleted")
			return ctrl.Result{}, nil
		}

		return res, fmt.Errorf("failed to get the requested %s KMMO CR: %w", req.NamespacedName, err)
	}

	if aic.GetDeletionTimestamp() != nil {
		err = r.helper.finalizeAIC(ctx, aic)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to finalize AIC %s: %v", req.NamespacedName, err)
		}
		return ctrl.Result{}, nil
	}

	err = r.helper.setFinalizer(ctx, aic)
	if err != nil {
		return res, fmt.Errorf("failed to set finalizer for AIC %s: %v", req.NamespacedName, err)
	}

	logger.Info("start NFR reconciliation")
        err = r.helper.handleAICNFDRule(ctx, aic)
        if err != nil {
                return res, fmt.Errorf("failed to handle NFR creation %s: %v", req.NamespacedName, err)
        }

	logger.Info("start build configmap reconciliation")
	// Always want to have the ConfigMap that can build module images
	err = r.helper.handleBuildConfigMap(ctx, aic, false)
	if err != nil {
		return res, fmt.Errorf("failed to handle build ConfigMap for DeviceConfig %s: %v", req.NamespacedName, err)
	}
	// but if we are enabling inTreeModules, we need one that creates blanks
	if aic.Spec.UseInTreeModules {
		err = r.helper.handleBuildConfigMap(ctx, aic, true)
		if err != nil {
			return res, fmt.Errorf("failed to handle build ConfigMap for DeviceConfig %s: %v", req.NamespacedName, err)
		}
	}

	logger.Info("start KMM reconciliation")
	err = r.helper.handleKMMModule(ctx, aic, aicv1.Qaic_loaded)
	if err != nil {
		return res, fmt.Errorf("failed to handle KMM module for AIC %s: %v", req.NamespacedName, err)
	}

	// TODO Metrics
	// TODO Handle Deletion

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AICReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&aicv1.AIC{}).
		Owns(&kmmv1beta1.Module{}).
		Complete(r)
}

type AICReconcilerHelperAPI interface {
	getRequestedAIC(ctx context.Context, namespacedName types.NamespacedName) (*aicv1.AIC, error)
	finalizeAIC(ctx context.Context, aic *aicv1.AIC) error
	setFinalizer(ctx context.Context, aic *aicv1.AIC) error
	handleBuildConfigMap(ctx context.Context, devConfig *aicv1.AIC, useInTree bool) error
	handleKMMModule(ctx context.Context, devConfig *aicv1.AIC, loadedMods aicv1.LoadedModules) error
	handleAICNFDRule(ctx context.Context, aic *aicv1.AIC) error
}

type AICReconcilerHelper struct {
	client     client.Client
	kmmHandler kmmmodule.KMMModuleAPI
	nfdHandler nfdrule.NFDRuleAPI
}

func newAICReconcilerHelper(client client.Client,
	kmmHandler kmmmodule.KMMModuleAPI, nfdHandler nfdrule.NFDRuleAPI) AICReconcilerHelperAPI {
	return &AICReconcilerHelper{
		client:     client,
		kmmHandler: kmmHandler,
		nfdHandler: nfdHandler,
	}
}
func (aicrh *AICReconcilerHelper) getRequestedAIC(ctx context.Context, namespacedName types.NamespacedName) (*aicv1.AIC, error) {
	aic := aicv1.AIC{}

	if err := aicrh.client.Get(ctx, namespacedName, &aic); err != nil {
		return nil, fmt.Errorf("failed to get AIC %s: %w", namespacedName, err)
	}
	return &aic, nil
}

func (aicrh *AICReconcilerHelper) setFinalizer(ctx context.Context, aic *aicv1.AIC) error {
	if controllerutil.ContainsFinalizer(aic, aicFinalizer) {
		return nil
	}

	aicCopy := aic.DeepCopy()
	controllerutil.AddFinalizer(aic, aicFinalizer)
	return aicrh.client.Patch(ctx, aic, client.MergeFrom(aicCopy))
}

func (aicrh *AICReconcilerHelper) finalizeAIC(ctx context.Context, aic *aicv1.AIC) error {
	var err error
	logger := log.FromContext(ctx)

	mod := kmmv1beta1.Module{}
	deleted := make([]error, 0, 2)
	faults := make([]error, 0, 2)
	nsName := types.NamespacedName{
		Namespace: aic.Namespace,
		Name:      aic.Name,
	}
	err = aicrh.client.Get(ctx, nsName, &mod)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			deleted = append(deleted, err)
		} else {
			faults = append(faults, fmt.Errorf("failed to get the requested Module %s: %w", nsName, err))
		}
	} else {
		logger.Info("deleting KMM Module", "module", nsName)
		if err = aicrh.client.Delete(ctx, &mod); client.IgnoreNotFound(err) != nil {
			faults = append(faults, err)
		}
	}
        //Delete NFR owned by AIC.
	nfrObj := nfr.NodeFeatureRule{}
	nsName = types.NamespacedName{
		Namespace: aic.Namespace,
		Name: "qcom-aic-nfr",
	}
	err = aicrh.client.Get(ctx, nsName, &nfrObj)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			deleted = append(deleted, err)
		} else {
			faults = append(faults, fmt.Errorf("failed to get the requested NFR %s: %w", nsName, err))
		}
	} else {
		logger.Info("deleting NFR CR", "NFR", nsName)
		if err = aicrh.client.Delete(ctx, &nfrObj); client.IgnoreNotFound(err) != nil {
			faults = append(faults, err)
		}
	}

	err = errors.Join(faults...)

	//remove finalizer only if no faults occurred during removal
	//len==2, because 1 for Module and 1 for NFR.
	if len(deleted) == 2 && err == nil {
		logger.Info("Module & NFR already deleted, removing finalizer", "Module, NFR", aic.Name)
		aicCopy := aic.DeepCopy()
		controllerutil.RemoveFinalizer(aic, aicFinalizer)
		return aicrh.client.Patch(ctx, aic, client.MergeFrom(aicCopy))
	}

	return err
}

func (aicrh *AICReconcilerHelper) handleBuildConfigMap(ctx context.Context, aic *aicv1.AIC, useInTree bool) error {
	buildDockerfileCM := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: aic.Namespace,
			Name:      getDockerfileCMName(aic, useInTree),
		},
	}
	logger := log.FromContext(ctx)
	opRes, err := controllerutil.CreateOrPatch(ctx, aicrh.client, buildDockerfileCM, func() error {
		return aicrh.kmmHandler.SetBuildConfigMapAsDesired(buildDockerfileCM, aic, useInTree)
	})

	if err == nil {
		logger.Info("Reconciled KMM build dockerfile ConfigMap", "name", buildDockerfileCM.Name, "result", opRes)
	}

	return err
}

func (aicrh *AICReconcilerHelper) handleKMMModule(ctx context.Context, aic *aicv1.AIC, loadedMods aicv1.LoadedModules) error {
	if loadedMods > aicv1.None_loaded {
		return fmt.Errorf("invalid loadedModule type = %d", loadedMods)
	}

	kmmMod := &kmmv1beta1.Module{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: aic.Namespace,
			Name:      aic.Name,
		},
	}
	logger := log.FromContext(ctx)
	opRes, err := controllerutil.CreateOrPatch(ctx, aicrh.client, kmmMod, func() error {
		return aicrh.kmmHandler.SetKMMModuleAsDesired(kmmMod, aic, loadedMods)
	})

	if err == nil {
		logger.Info("Reconciled KMM Module", "name", kmmMod.Name, "result", opRes)
	}

	return err
}

func (aicrh *AICReconcilerHelper) handleAICNFDRule(ctx context.Context, aic *aicv1.AIC) error {

	nfrObj := &nfr.NodeFeatureRule{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: aic.Namespace,
			Name:      "qcom-aic-nfr",
		},
	}
        logger := log.FromContext(ctx)
         opRes, err := controllerutil.CreateOrPatch(ctx, aicrh.client, nfrObj, func() error {
                return aicrh.nfdHandler.SetNFRasDesired(nfrObj, aic)
        })
         if err != nil {
                logger.Info("Reconciled NFR", "name", nfrObj.Name, "result", opRes)
                 return err
         }
        return nil

}
func getDockerfileCMName(aic *aicv1.AIC, useInTree bool) string {
	if useInTree {
		return "dockerfile-intree-" + aic.Name
	} else {
		return "dockerfile-" + aic.Name
	}
}
