package utils

type VideoJob struct {
	VideoID      string `json:"video_id"`
	S3Path       string `json:"s3_path"`
	OriginalName string `json:"original_filename"`
	Bucket       string `json:"bucket"`
}
