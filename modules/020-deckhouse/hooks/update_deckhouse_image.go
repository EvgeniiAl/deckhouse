/*
Copyright 2021 Flant JSC

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

package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	"github.com/deckhouse/deckhouse/go_lib/hooks/update"
	"github.com/deckhouse/deckhouse/modules/020-deckhouse/hooks/internal/v1alpha1"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/deckhouse/update_deckhouse_image",
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "update_deckhouse_image",
			Crontab: "*/15 * * * * *",
		},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "deckhouse_pod",
			ApiVersion: "v1",
			Kind:       "Pod",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "deckhouse",
				},
			},
			ExecuteHookOnEvents:          pointer.BoolPtr(false),
			ExecuteHookOnSynchronization: pointer.BoolPtr(false),
			FilterFunc:                   filterDeckhousePod,
		},
		{
			Name:                         "releases",
			ApiVersion:                   "deckhouse.io/v1alpha1",
			Kind:                         "DeckhouseRelease",
			ExecuteHookOnSynchronization: pointer.BoolPtr(false),
			FilterFunc:                   filterDeckhouseRelease,
		},
	},
}, dependency.WithExternalDependencies(updateDeckhouse))

type deckhousePodInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Image     string `json:"image"`
	ImageID   string `json:"imageID"`
	Ready     bool   `json:"ready"`
}

type deckhouseUpdater struct {
	manualMode bool // Manual update mode activated

	releases                   []deckhouseRelease
	hasPatchRelease            bool // next release is a patch
	totalPendingManualReleases uint
}

func newDeckhouseUpdater(input *go_hook.HookInput) *deckhouseUpdater {
	approvalMode := input.Values.Get("deckhouse.update.mode").String()

	return &deckhouseUpdater{
		manualMode: approvalMode == "Manual",
	}
}

func (du *deckhouseUpdater) patchReleaseStatus(input *go_hook.HookInput, release deckhouseRelease, ts time.Time) deckhouseRelease {
	var statusChanged bool

	statusPatch := statusPatch{
		Phase:          release.Phase,
		Approved:       release.StatusApproved,
		TransitionTime: ts,
	}

	// when DeckhouseRelease just created and doesn't have any status
	if release.Phase == "" {
		statusPatch.Phase = v1alpha1.PhasePending
		statusPatch.TransitionTime = ts
		statusChanged = true
	}

	if statusPatch.Phase == v1alpha1.PhasePending {
		// check and set .status.approved for pending releases
		if du.manualMode && !release.ManualApproved {
			statusPatch.Approved = false
			du.totalPendingManualReleases++
			if release.StatusApproved {
				statusChanged = true
			}
		} else {
			statusPatch.Approved = true
			if !release.StatusApproved {
				statusChanged = true
			}
		}
	}

	if statusChanged {
		input.PatchCollector.MergePatch(statusPatch, "deckhouse.io/v1alpha1", "DeckhouseRelease", "", release.Name, object_patch.WithSubresource("/status"))

		release.StatusApproved = statusPatch.Approved
		release.Phase = statusPatch.Phase
	}

	return release
}

func (du *deckhouseUpdater) FetchAndPrepareReleases(input *go_hook.HookInput) {
	snap := input.Snapshots["releases"]
	if len(snap) == 0 {
		return
	}

	now := time.Now()

	releases := make([]deckhouseRelease, 0, len(snap))
	var currentReleaseIndex = -1
	var currentReleaseVersion *semver.Version

	for _, rl := range snap {
		release := rl.(deckhouseRelease)

		release = du.patchReleaseStatus(input, release, now)

		releases = append(releases, release)
	}

	sort.Sort(byVersion(releases))

	// detect if next release is a patch version
	for i, release := range releases {
		if release.Phase == v1alpha1.PhaseDeployed {
			currentReleaseIndex = i
			currentReleaseVersion = release.Version
			break
		}
	}

	if currentReleaseVersion != nil {
		if len(releases) > currentReleaseIndex+1 {
			newRelease := releases[currentReleaseIndex+1]
			if newRelease.Version.Major() == currentReleaseVersion.Major() && newRelease.Version.Minor() == currentReleaseVersion.Minor() {
				du.hasPatchRelease = true
			}
		}
	}

	du.releases = releases
}

func updateDeckhouse(input *go_hook.HookInput, dc dependency.Container) error {
	if !input.Values.Exists("deckhouse.releaseChannel") {
		// dev upgrade - by tag
		return tagUpdate(input, dc)
	}

	// production upgrade
	updater := newDeckhouseUpdater(input)

	updater.FetchAndPrepareReleases(input)

	// update windows works only for Auto deployment mode
	if !updater.manualMode {
		windows, exists := input.Values.GetOk("deckhouse.update.windows")
		if exists {
			if updater.hasPatchRelease {
				// patch release does not respect update windows
				return updater.releaseChannelUpdate(input)
			}

			updatePermitted, err := isUpdatePermitted([]byte(windows.Raw))
			if err != nil {
				return fmt.Errorf("update windows configuration is not valid: %s", err)
			}
			if !updatePermitted {
				input.LogEntry.Debug("Deckhouse update does not get into update windows. Skipping")
				return nil
			}
		}
	}

	return updater.releaseChannelUpdate(input)
}

// used also in check_deckhouse_release.go
func filterDeckhouseRelease(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var release v1alpha1.DeckhouseRelease

	err := sdk.FromUnstructured(unstructured, &release)
	if err != nil {
		return nil, err
	}

	var hasSuspendAnnotation bool
	if v, ok := release.Annotations["release.deckhouse.io/suspended"]; ok {
		if v == "true" {
			hasSuspendAnnotation = true
		}
	}

	return deckhouseRelease{
		Name:                 release.Name,
		Version:              semver.MustParse(release.Spec.Version),
		ApplyAfter:           release.Spec.ApplyAfter,
		Requirements:         release.Spec.Requirements,
		Phase:                release.Status.Phase,
		ManualApproved:       release.Approved,
		StatusApproved:       release.Status.Approved,
		HasSuspendAnnotation: hasSuspendAnnotation,
	}, nil
}

type deckhouseRelease struct {
	Name         string
	Version      *semver.Version
	ApplyAfter   *time.Time
	Requirements map[string]string

	Phase          string
	ManualApproved bool
	StatusApproved bool

	HasSuspendAnnotation bool
}

type byVersion []deckhouseRelease

func (a byVersion) Len() int {
	return len(a)
}
func (a byVersion) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}
func (a byVersion) Less(i, j int) bool {
	return a[i].Version.LessThan(a[j].Version)
}

func filterDeckhousePod(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var pod corev1.Pod
	err := sdk.FromUnstructured(unstructured, &pod)
	if err != nil {
		return nil, err
	}

	var imageName, imageID string

	if len(pod.Spec.Containers) > 0 {
		imageName = pod.Spec.Containers[0].Image
	}

	var ready bool

	if len(pod.Status.ContainerStatuses) > 0 {
		imageID = pod.Status.ContainerStatuses[0].ImageID
		ready = pod.Status.ContainerStatuses[0].Ready
	}

	return deckhousePodInfo{
		Image:     imageName,
		ImageID:   imageID,
		Name:      pod.Name,
		Namespace: pod.Namespace,
		Ready:     ready,
	}, nil
}

// releaseChannelUpdate update with previously set release channel when CR DeckhouseRelease exists
func (du *deckhouseUpdater) releaseChannelUpdate(input *go_hook.HookInput) error {
	input.MetricsCollector.Expire("d8_releases")

	snap := input.Snapshots["deckhouse_pod"]
	if len(snap) == 0 {
		return nil
	}

	// upgrade only when current release is ready.
	// skip it for patches.
	deckhousePod := snap[0].(deckhousePodInfo)
	if !du.hasPatchRelease && !deckhousePod.Ready {
		input.LogEntry.Info("Deckhouse is not ready. Skipping upgrade")
		return nil
	}

	now := time.Now()

	currentDeployedReleaseIndex := -1

	for i, rl := range du.releases {
		switch rl.Phase {
		// "Deployed" shows only Actual (current) release. All previous releases are marked as Outdated
		// It's much more comfortable to observe DeckhouseReleases like this because by default they are sorted by Name
		// and sometimes it's a bit weird for semver names. This statuses shows you the real view of releases
		case v1alpha1.PhaseOutdated, v1alpha1.PhaseSuspended:
		// pass

		case v1alpha1.PhasePending:
			// skip suspended releases
			if rl.HasSuspendAnnotation {
				sp := statusPatch{
					Phase:          v1alpha1.PhaseSuspended,
					Message:        "",
					TransitionTime: now,
				}
				input.PatchCollector.MergePatch(sp, "deckhouse.io/v1alpha1", "DeckhouseRelease", "", rl.Name, object_patch.WithSubresource("/status"))
				continue
			}

			if i < currentDeployedReleaseIndex {
				// some old release, for example - when downgrade the release channel
				// mark it as Outdated
				sp := statusPatch{
					Phase:          v1alpha1.PhaseOutdated,
					Message:        "",
					TransitionTime: now,
				}
				input.PatchCollector.MergePatch(sp, "deckhouse.io/v1alpha1", "DeckhouseRelease", "", rl.Name, object_patch.WithSubresource("/status"))
				continue
			}

			if rl.ApplyAfter != nil {
				if time.Now().UTC().Before(*rl.ApplyAfter) {
					input.LogEntry.Infof("Release %s is postponed by canary process. Waiting", rl.Name)
					return nil
				}
			}

			// always deploy Patch releases
			if !rl.StatusApproved && !du.hasPatchRelease {
				input.LogEntry.Infof("Release %s is waiting for manual approval", rl.Name)
				input.MetricsCollector.Set("d8_release_waiting_manual", float64(du.totalPendingManualReleases), map[string]string{"name": rl.Name}, metrics.WithGroup("d8_releases"))
				return nil
			}

			// apply patch - update deployment
			applied := applyRelease(input, rl, now)
			if !applied {
				return nil
			}

			// mark previous release as outdated
			if currentDeployedReleaseIndex != -1 {
				currentDeployedRelease := du.releases[currentDeployedReleaseIndex]
				sp := statusPatch{
					Phase:          v1alpha1.PhaseOutdated,
					Approved:       currentDeployedRelease.StatusApproved,
					Message:        "",
					TransitionTime: now,
				}
				input.PatchCollector.MergePatch(sp, "deckhouse.io/v1alpha1", "DeckhouseRelease", "", currentDeployedRelease.Name, object_patch.WithSubresource("/status"))
			}
			return nil

		case v1alpha1.PhaseDeployed:
			if i == len(du.releases)-1 {
				// latest release, don't update
				return nil
			}
			currentDeployedReleaseIndex = i
		}
	}

	if currentDeployedReleaseIndex == -1 {
		// self-healing, if deployed release was deleted
		// no deployed releases found - deploy first pending release
		for _, rl := range du.releases {
			if rl.Phase == v1alpha1.PhasePending {
				_ = applyRelease(input, rl, now)
				return nil
			}
		}
	}

	return nil
}

func checkReleaseRequirements(input *go_hook.HookInput, rl deckhouseRelease, ts time.Time) bool {
	for key, value := range rl.Requirements {
		passed, err := requirements.CheckRequirement(key, value, input.Values)
		if !passed {
			msg := fmt.Sprintf("%q requirement for DeckhouseRelease %q not met: %s", key, rl.Version, err)
			st := statusPatch{
				Phase:          v1alpha1.PhasePending,
				Message:        msg,
				Approved:       true,
				TransitionTime: ts,
			}
			input.PatchCollector.MergePatch(st, "deckhouse.io/v1alpha1", "DeckhouseRelease", "", rl.Name, object_patch.WithSubresource("/status"))
			return false
		}
	}

	return true
}

func applyRelease(input *go_hook.HookInput, rl deckhouseRelease, ts time.Time) (applied bool) {
	// check release dependencies
	passed := checkReleaseRequirements(input, rl, ts)
	if !passed {
		input.MetricsCollector.Set("d8_release_blocked", 1, map[string]string{"name": rl.Name}, metrics.WithGroup("d8_releases"))
		return false
	}

	input.LogEntry.Infof("Applying release %s", rl.Name)

	repo := input.Values.Get("global.modulesImages.registry").String()

	st := statusPatch{
		Phase:          v1alpha1.PhaseDeployed,
		Approved:       true,
		Message:        "",
		TransitionTime: ts,
	}
	input.PatchCollector.MergePatch(st, "deckhouse.io/v1alpha1", "DeckhouseRelease", "", rl.Name, object_patch.WithSubresource("/status"))
	// patch deckhouse deployment is faster then set internal values and then upgrade by helm
	// we can set "deckhouse.internal.currentReleaseImageName" value but lets left it this way
	input.PatchCollector.Filter(func(u *unstructured.Unstructured) (*unstructured.Unstructured, error) {
		var depl appsv1.Deployment
		err := sdk.FromUnstructured(u, &depl)
		if err != nil {
			return nil, err
		}

		depl.Spec.Template.Spec.Containers[0].Image = repo + ":" + rl.Version.Original()

		return sdk.ToUnstructured(&depl)
	}, "apps/v1", "Deployment", "d8-system", "deckhouse")

	return true
}

func isUpdatePermitted(windowsData []byte) (bool, error) {
	if len(windowsData) == 0 {
		return true, nil
	}

	now := time.Now()

	if os.Getenv("D8_IS_TESTS_ENVIRONMENT") != "" {
		now = time.Date(2021, 01, 01, 13, 30, 00, 00, time.UTC)
	}

	windows, err := update.FromJSON(windowsData)
	if err != nil {
		return false, err
	}

	return windows.IsAllowed(now), nil
}

// tagUpdate update by tag, in dev mode or specified image
func tagUpdate(input *go_hook.HookInput, dc dependency.Container) error {
	snap := input.Snapshots["deckhouse_pod"]
	if len(snap) == 0 {
		return nil
	}

	deckhousePod := snap[0].(deckhousePodInfo)
	if deckhousePod.Image == "" && deckhousePod.ImageID == "" {
		// pod is restarting or something like that, try more in a 15 seconds
		return nil
	}

	if deckhousePod.Image == "" || deckhousePod.ImageID == "" {
		input.LogEntry.Debug("Deckhouse pod is not ready. Try to update later")
		return nil
	}

	idSplitIndex := strings.LastIndex(deckhousePod.ImageID, "@")
	if idSplitIndex == -1 {
		return fmt.Errorf("image hash not found: %s", deckhousePod.ImageID)
	}
	imageHash := deckhousePod.ImageID[idSplitIndex+1:]

	imageSplitIndex := strings.LastIndex(deckhousePod.Image, ":")
	if imageSplitIndex == -1 {
		return fmt.Errorf("image tag not found: %s", deckhousePod.Image)
	}
	repo := deckhousePod.Image[:imageSplitIndex]
	tag := deckhousePod.Image[imageSplitIndex+1:]

	regClient, err := dc.GetRegistryClient(repo, cr.WithCA(getCA(input)), cr.WithInsecureSchema(isHTTP(input)))
	if err != nil {
		input.LogEntry.Errorf("Registry (%s) client init failed: %s", repo, err)
		return nil
	}

	input.MetricsCollector.Inc("deckhouse_registry_check_total", map[string]string{})
	input.MetricsCollector.Inc("deckhouse_kube_image_digest_check_total", map[string]string{})

	repoDigest, err := regClient.Digest(tag)
	if err != nil {
		input.MetricsCollector.Inc("deckhouse_registry_check_errors_total", map[string]string{})
		input.LogEntry.Errorf("Registry (%s) get digest failed: %s", repo, err)
		return nil
	}

	input.MetricsCollector.Set("deckhouse_kube_image_digest_check_success", 1.0, map[string]string{})

	if strings.TrimSpace(repoDigest) == strings.TrimSpace(imageHash) {
		return nil
	}

	input.LogEntry.Info("New deckhouse image found. Restarting.")

	input.PatchCollector.Delete("v1", "Pod", deckhousePod.Namespace, deckhousePod.Name)

	return nil
}

type statusPatch v1alpha1.DeckhouseReleaseStatus

func (sp statusPatch) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{
		"status": v1alpha1.DeckhouseReleaseStatus(sp),
	}

	return json.Marshal(m)
}
