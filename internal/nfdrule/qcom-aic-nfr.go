/*
Copyright (c) 2025 Qualcomm Innovation Center, Inc. All rights reserved.
SPDX-License-Identifier: BSD-3-Clause-Clear.
*/

package nfdrule

import (
	_ "embed"
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	aicv1 "github.com/quic/aic-operator/api/v1"
	nfr "github.com/openshift/cluster-nfd-operator/api/v1alpha1"
)

const (
	qcom_aic_nfrmanifest = "/opt/aic-manifests/qcom-aic-nfr.yaml"
)

type NFDRuleAPI interface {
	SetNFRasDesired(nfrObj *nfr.NodeFeatureRule, aic *aicv1.AIC) error
}

type nfdRule struct {
	client client.Client
	scheme *runtime.Scheme
}

func NewNFDRule(client client.Client, scheme *runtime.Scheme) NFDRuleAPI {
	return &nfdRule{
		client: client,
		scheme: scheme,
	}
}

func (nfrstruct *nfdRule) SetNFRasDesired(nfrObj *nfr.NodeFeatureRule, aic *aicv1.AIC) error {
	err := parseNFR_Manifest(nfrObj, aic)
	if err != nil {
		return fmt.Errorf("failed to set NodeFeatureRule: %v", err)
	}
	return controllerutil.SetControllerReference(aic, nfrObj, nfrstruct.scheme)
}

func parseNFR_Manifest(nfrObj *nfr.NodeFeatureRule, aic *aicv1.AIC) error {
	raw_manifest, err := ioutil.ReadFile(qcom_aic_nfrmanifest)
        if err != nil {
                return fmt.Errorf("Error encountered while reading NFR manifest %s : %v", qcom_aic_nfrmanifest, err)
        }
	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)
	regex, _ := regexp.Compile(`\b(\w*kind:\w*)\B.*\b`)
	kind := strings.TrimSpace(strings.Split(regex.FindString(string(raw_manifest)), ":")[1])
	fmt.Println("Resource identified kind:", kind)
	_, _, err = s.Decode(raw_manifest, nil, nfrObj)
	if err != nil {
		return fmt.Errorf("Error encountered while decoding %s resource in manifest %s: %v", "NodeFeatureRule", qcom_aic_nfrmanifest, err)
	}
	nfrObj.ObjectMeta.Namespace = aic.Namespace
	return nil
}
