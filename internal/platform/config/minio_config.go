package config

import "time"

type MinioConfig struct {
	Endpoint           string
	AccessKey          string
	SecretKey          string
	UseSSL             bool
	PresignedUrlExpiry time.Duration
	BucketAvatars      string
	BucketBooks        string
}

func loadMinioConfig() MinioConfig {
	return MinioConfig{
		Endpoint:           mustGetEnv("MINIO_ENDPOINT"),
		AccessKey:          mustGetEnv("MINIO_ACCESS_KEY"),
		SecretKey:          mustGetEnv("MINIO_SECRET_KEY"),
		UseSSL:             getEnvAsBool("MINIO_USE_SSL", false),
		PresignedUrlExpiry: time.Duration(getEnvAsInt("MINIO_PRESIGNED_URL_EXPIRY_MINUTES", 60)) * time.Minute,
		BucketAvatars:      mustGetEnv("MINIO_BUCKET_AVATARS"),
		BucketBooks:        mustGetEnv("MINIO_BUCKET_BOOKS"),
	}
}
