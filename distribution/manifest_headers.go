package distribution

import (
	"github.com/docker/distribution"
	"github.com/docker/distribution/reference"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	"net/http"
	"sync"
)

// distributionRepositoryWithManifestInfo is a distribution.Repository implementation wrapper which keeps track on the
// manifest information (the tag and the digests) being pushed / pulled. It also acts as transport.RequestModifier to
// modify requests, adding headers with the manifest tag and digests.
//
// Multiple digests are collected e.g. in the scenario of pulling a manifest list. In this case, the digests are
// accumulated every time a specific manifest is resolved. The leftmost (i.e. index 0) digest is expected to be the
// first requested manifest, the rightmost (i.e. last index) is the last resolved manifest in the chain.
type distributionRepositoryWithManifestInfo struct {
	distribution.Repository
	manifestInfo struct {
		tag     string
		digests []string
	}
	mutex sync.RWMutex
}

var _ distribution.Repository = (*distributionRepositoryWithManifestInfo)(nil)

func (r *distributionRepositoryWithManifestInfo) ModifyRequest(req *http.Request) error {
	logrus.Tracef("distributionRepositoryWithManifestInfo.ModifyRequest: %s %s", req.Method, req.URL)
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	info := r.manifestInfo
	if info.tag != "" {
		logrus.Tracef("Adding manifest header - Docker-Manifest-Tag: %s", info.tag)
		req.Header.Set("Docker-Manifest-Tag", info.tag)
	}
	if len(info.digests) > 0 {
		logrus.Tracef("Adding manifest header - Docker-Manifest-Digest: %s", info.digests)
		for _, value := range info.digests {
			req.Header.Add("Docker-Manifest-Digest", value)
		}
	}
	return nil
}

// update the manifest info kept by this instance according to the given named ref and the optional list of
// digests.
//
// Note - if both ref is a reference.Digested and digests is not empty, then the digests have priority over the digest
// from the ref.
func (r *distributionRepositoryWithManifestInfo) update(ref reference.Named, digests ...digest.Digest) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	info := r.manifestInfo
	if tagged, ok := ref.(reference.Tagged); ok {
		info.tag = tagged.Tag()
		logrus.Tracef("distributionRepositoryWithManifestInfo: updated tag='%s' (from ref: %#v)", info.tag, ref)
	}
	if digested, ok := ref.(reference.Digested); ok {
		info.digests = []string{digested.Digest().String()}
		logrus.Tracef("distributionRepositoryWithManifestInfo: updated digests='%+v' (from ref: %#v)", info.digests, ref)
	}
	// Explicit digests have priority over the digest from the ref
	if len(digests) > 0 {
		info.digests = make([]string, 0, len(digests))
		for _, dgst := range digests {
			if dgst == "" {
				continue
			}
			info.digests = append(info.digests, dgst.String())
		}
		logrus.Tracef("distributionRepositoryWithManifestInfo: updated digests='%+v'", info.digests)
	}
	r.manifestInfo = info
}

// addDigest adds a digest to the list of kept digests by this instance, as the last digest
func (r *distributionRepositoryWithManifestInfo) addDigest(dgst digest.Digest) {
	if dgst == "" {
		return
	}
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.manifestInfo.digests = append(r.manifestInfo.digests, dgst.String())
	logrus.Tracef("distributionRepositoryWithManifestInfo: updated digests='%+v' (added: '%s')", r.manifestInfo.digests, dgst)
}

// prepareRestoreInfo returns a function which can be used to restore the manifest info to the current state. This
// function can be called multiple times, which will result with the same state every time.
func (r *distributionRepositoryWithManifestInfo) prepareRestoreInfo() func() {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	info := r.manifestInfo
	return func() {
		r.mutex.Lock()
		defer r.mutex.Unlock()
		r.manifestInfo = info
		logrus.Tracef("distributionRepositoryWithManifestInfo: restored manifest info='%+v'", r.manifestInfo)
	}
}

// updateRepoWithManifestInfo safely calls distributionRepositoryWithManifestInfo.update if repo is a
// distributionRepositoryWithManifestInfo
func updateRepoWithManifestInfo(repo distribution.Repository, ref reference.Named, dgst ...digest.Digest) {
	if r, ok := repo.(*distributionRepositoryWithManifestInfo); ok {
		r.update(ref, dgst...)
	}
}

// prepareRestoreRepoWithManifestInfo safely calls distributionRepositoryWithManifestInfo.prepareRestoreInfo if repo is a
//// distributionRepositoryWithManifestInfo, otherwise it returns a no-op function
func prepareRestoreRepoWithManifestInfo(repo distribution.Repository) func() {
	if r, ok := repo.(*distributionRepositoryWithManifestInfo); ok {
		return r.prepareRestoreInfo()
	}
	return func() {} // no-op
}

// addDigestToRepoWithManifestInfo safely calls distributionRepositoryWithManifestInfo.addDigest if repo is a
// distributionRepositoryWithManifestInfo
func addDigestToRepoWithManifestInfo(repo distribution.Repository, dgst digest.Digest) {
	if r, ok := repo.(*distributionRepositoryWithManifestInfo); ok {
		r.addDigest(dgst)
	}
}
