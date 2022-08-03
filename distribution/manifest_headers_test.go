package distribution

import (
	"github.com/docker/distribution/reference"
	"github.com/opencontainers/go-digest"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDistributionRepositoryWithManifestInfo_ModifyRequest(t *testing.T) {
	assertExpectedHeaders := func(t *testing.T, req *http.Request, tag string, digests ...string) {
		var expectedTags []string = nil
		if tag != "" {
			expectedTags = []string{tag}
		}
		tagValues := req.Header.Values("Docker-Manifest-Tag")
		assert.Check(t, cmp.DeepEqual(expectedTags, tagValues), "manifest tag values in header not as expected")
		expectedDigests := digests
		digestValues := req.Header.Values("Docker-Manifest-Digest")
		assert.Check(t, cmp.DeepEqual(expectedDigests, digestValues), "manifest digest values in header not as expected")
	}

	repo := &distributionRepositoryWithManifestInfo{}
	newRequest := func() *http.Request { return httptest.NewRequest(http.MethodGet, "https://www.example.com", nil) }
	refName, _ := reference.WithName("foo")
	refWithTag1, _ := reference.WithTag(refName, "1.0")
	refWithTag2, _ := reference.WithTag(refName, "2.0")
	dgst1, _ := digest.Parse("sha256:12345678901234567890123456789012")
	dgst2, _ := digest.Parse("sha256:23456789012345678901234567890123")
	dgst3, _ := digest.Parse("sha256:34567890123456789012345678901234")
	dgst4, _ := digest.Parse("sha256:45678901234567890123456789012345")
	dgst5, _ := digest.Parse("sha256:56789012345678901234567890123456")
	refWithDigest1, _ := reference.WithDigest(refName, dgst1)
	refWithDigest3, _ := reference.WithDigest(refName, dgst3)

	t.Run("initial values", func(t *testing.T) {
		req := newRequest()
		err := repo.ModifyRequest(req)
		assert.NilError(t, err)
		assertExpectedHeaders(t, req, "")
	})

	t.Run("update only ref tag", func(t *testing.T) {
		req := newRequest()
		repo.update(refWithTag1)
		err := repo.ModifyRequest(req)
		assert.NilError(t, err)
		assertExpectedHeaders(t, req, "1.0")
	})

	t.Run("update only digest from ref", func(t *testing.T) {
		req := newRequest()
		repo.update(refWithDigest1)
		err := repo.ModifyRequest(req)
		assert.NilError(t, err)
		assertExpectedHeaders(t, req, "1.0", dgst1.String())
	})

	t.Run("update both ref tag and explicit digest", func(t *testing.T) {
		req := newRequest()
		repo.update(refWithTag2, dgst2)
		err := repo.ModifyRequest(req)
		assert.NilError(t, err)
		assertExpectedHeaders(t, req, "2.0", dgst2.String())
	})

	t.Run("update with both ref digest and explicit digest", func(t *testing.T) {
		req := newRequest()
		repo.update(refWithDigest3, dgst4)
		err := repo.ModifyRequest(req)
		assert.NilError(t, err)
		assertExpectedHeaders(t, req, "2.0", dgst4.String())
	})

	restore := repo.prepareRestoreInfo()

	t.Run("add digest", func(t *testing.T) {
		req := newRequest()
		repo.addDigest(dgst5)
		err := repo.ModifyRequest(req)
		assert.NilError(t, err)
		assertExpectedHeaders(t, req, "2.0", dgst4.String(), dgst5.String())
	})

	t.Run("restore", func(t *testing.T) {
		req := newRequest()
		restore()
		err := repo.ModifyRequest(req)
		assert.NilError(t, err)
		assertExpectedHeaders(t, req, "2.0", dgst4.String())
	})
}
