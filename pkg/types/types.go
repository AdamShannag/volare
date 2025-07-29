package types

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const DefaultNumberOfWorkers = 2

type SourceType string

const (
	SourceTypeHTTP   SourceType = "http"
	SourceTypeS3     SourceType = "s3"
	SourceTypeGITHUB SourceType = "github"
	SourceTypeGITLAB SourceType = "gitlab"
)

type VolarePopulator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec VolarePopulatorSpec `json:"spec"`
}

type VolarePopulatorSpec struct {
	Sources []Source `json:"sources"`
	Workers *int     `json:"workers,omitempty"`
}

type Source struct {
	Type       SourceType `json:"type"`
	TargetPath string     `json:"targetPath"`

	Http   *HttpOptions   `json:"http,omitempty"`
	Gitlab *GitlabOptions `json:"gitlab,omitempty"`
	GitHub *GitHubOptions `json:"github,omitempty"`
	S3     *S3Options     `json:"s3,omitempty"`
}

type HttpOptions struct {
	URI     string            `json:"uri,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

type GitlabOptions struct {
	Host    string `json:"host"`
	Project string `json:"project"`
	Ref     string `json:"ref"`
	Path    string `json:"path"`
	Token   string `json:"token,omitempty"`
	Workers *int   `json:"workers,omitempty"`
}

type GitHubOptions struct {
	Owner   string `json:"owner"`
	Repo    string `json:"repo"`
	Ref     string `json:"ref"`
	Path    string `json:"path"`
	Token   string `json:"token,omitempty"`
	Workers *int   `json:"workers,omitempty"`
}

type S3Options struct {
	Endpoint        string   `json:"endpoint"`
	Secure          bool     `json:"secure"`
	Bucket          string   `json:"bucket"`
	Paths           []string `json:"paths"`
	Region          string   `json:"region"`
	AccessKeyID     string   `json:"accessKeyId"`
	SecretAccessKey string   `json:"secretAccessKey"`
	SessionToken    string   `json:"sessionToken,omitempty"`
	Workers         *int     `json:"workers,omitempty"`
}
