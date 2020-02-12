/*
Copyright The KubeDB Authors.

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

package util

import (
	"encoding/json"
	"fmt"

	dba "kubedb.dev/apimachinery/apis/dba/v1alpha1"
	cs "kubedb.dev/apimachinery/client/clientset/versioned/typed/dba/v1alpha1"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/golang/glog"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	kutil "kmodules.xyz/client-go"
)

func CreateOrPatchElasticsearchModificationRequest(c cs.DbaV1alpha1Interface, meta metav1.ObjectMeta, transform func(*dba.ElasticsearchModificationRequest) *dba.ElasticsearchModificationRequest) (*dba.ElasticsearchModificationRequest, kutil.VerbType, error) {
	cur, err := c.ElasticsearchModificationRequests().Get(meta.Name, metav1.GetOptions{})
	if kerr.IsNotFound(err) {
		glog.V(3).Infof("Creating ElasticsearchModificationRequest %s.", meta.Name)
		out, err := c.ElasticsearchModificationRequests().Create(transform(&dba.ElasticsearchModificationRequest{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ElasticsearchModificationRequest",
				APIVersion: dba.SchemeGroupVersion.String(),
			},
			ObjectMeta: meta,
		}))
		return out, kutil.VerbCreated, err
	} else if err != nil {
		return nil, kutil.VerbUnchanged, err
	}
	return PatchElasticsearchModificationRequest(c, cur, transform)
}

func PatchElasticsearchModificationRequest(c cs.DbaV1alpha1Interface, cur *dba.ElasticsearchModificationRequest, transform func(*dba.ElasticsearchModificationRequest) *dba.ElasticsearchModificationRequest) (*dba.ElasticsearchModificationRequest, kutil.VerbType, error) {
	return PatchElasticsearchModificationRequestObject(c, cur, transform(cur.DeepCopy()))
}

func PatchElasticsearchModificationRequestObject(c cs.DbaV1alpha1Interface, cur, mod *dba.ElasticsearchModificationRequest) (*dba.ElasticsearchModificationRequest, kutil.VerbType, error) {
	curJson, err := json.Marshal(cur)
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}

	modJson, err := json.Marshal(mod)
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}

	patch, err := jsonpatch.CreateMergePatch(curJson, modJson)
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}
	if len(patch) == 0 || string(patch) == "{}" {
		return cur, kutil.VerbUnchanged, nil
	}
	glog.V(3).Infof("Patching ElasticsearchModificationRequest %s with %s.", cur.Name, string(patch))
	out, err := c.ElasticsearchModificationRequests().Patch(cur.Name, types.MergePatchType, patch)
	return out, kutil.VerbPatched, err
}

func TryUpdateElasticsearchModificationRequest(c cs.DbaV1alpha1Interface, meta metav1.ObjectMeta, transform func(*dba.ElasticsearchModificationRequest) *dba.ElasticsearchModificationRequest) (result *dba.ElasticsearchModificationRequest, err error) {
	attempt := 0
	err = wait.PollImmediate(kutil.RetryInterval, kutil.RetryTimeout, func() (bool, error) {
		attempt++
		cur, e2 := c.ElasticsearchModificationRequests().Get(meta.Name, metav1.GetOptions{})
		if kerr.IsNotFound(e2) {
			return false, e2
		} else if e2 == nil {
			result, e2 = c.ElasticsearchModificationRequests().Update(transform(cur.DeepCopy()))
			return e2 == nil, nil
		}
		glog.Errorf("Attempt %d failed to update ElasticsearchModificationRequest %s due to %v.", attempt, cur.Name, e2)
		return false, nil
	})

	if err != nil {
		err = fmt.Errorf("failed to update ElasticsearchModificationRequest %s after %d attempts due to %v", meta.Name, attempt, err)
	}
	return
}

func UpdateElasticsearchModificationRequestStatus(
	c cs.DbaV1alpha1Interface,
	in *dba.ElasticsearchModificationRequest,
	transform func(*dba.ElasticsearchModificationRequestStatus) *dba.ElasticsearchModificationRequestStatus,
) (result *dba.ElasticsearchModificationRequest, err error) {
	apply := func(x *dba.ElasticsearchModificationRequest) *dba.ElasticsearchModificationRequest {
		return &dba.ElasticsearchModificationRequest{
			TypeMeta:   x.TypeMeta,
			ObjectMeta: x.ObjectMeta,
			Spec:       x.Spec,
			Status:     *transform(in.Status.DeepCopy()),
		}
	}

	attempt := 0
	cur := in.DeepCopy()
	err = wait.PollImmediate(kutil.RetryInterval, kutil.RetryTimeout, func() (bool, error) {
		attempt++
		var e2 error
		result, e2 = c.ElasticsearchModificationRequests().UpdateStatus(apply(cur))
		if kerr.IsConflict(e2) {
			latest, e3 := c.ElasticsearchModificationRequests().Get(in.Name, metav1.GetOptions{})
			switch {
			case e3 == nil:
				cur = latest
				return false, nil
			case kutil.IsRequestRetryable(e3):
				return false, nil
			default:
				return false, e3
			}
		} else if err != nil && !kutil.IsRequestRetryable(e2) {
			return false, e2
		}
		return e2 == nil, nil
	})

	if err != nil {
		err = fmt.Errorf("failed to update status of ElasticsearchModificationRequest %s/%s after %d attempts due to %v", in.Namespace, in.Name, attempt, err)
	}
	return
}
