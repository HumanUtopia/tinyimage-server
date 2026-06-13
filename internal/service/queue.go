package service

import (
	"sync"

	"github.com/humanutopia/tinyimage-server/internal/model"
)

var (
	queue      = make(map[model.ProcessKey]string) // 任务key -> filename
	statusMap  = make(map[model.ProcessKey]string) // 任务key -> status
	queueMutex = &sync.Mutex{}
	processed  = make(map[model.ProcessKey]bool)   // 已处理过的任务key
)

func EnqueueTask(key model.ProcessKey, filename string) {
	queueMutex.Lock()
	defer queueMutex.Unlock()
	queue[key] = filename
	statusMap[key] = "queued"
}

func IsProcessed(key model.ProcessKey) bool {
	queueMutex.Lock()
	defer queueMutex.Unlock()
	return processed[key]
}

func GetQueueByMD5(md5Str string) []map[string]interface{} {
	queueMutex.Lock()
	defer queueMutex.Unlock()
	var items []map[string]interface{}
	for k, filename := range queue {
		if k.MD5 == md5Str {
			items = append(items, map[string]interface{}{
				"md5":      k.MD5,
				"filename": filename,
				"format":   k.Format,
				"quality":  k.Quality,
			})
		}
	}
	return items
}

func GetStatusByMD5(md5Str string) []map[string]interface{} {
	queueMutex.Lock()
	defer queueMutex.Unlock()
	var items []map[string]interface{}
	for k, stat := range statusMap {
		if k.MD5 == md5Str {
			items = append(items, map[string]interface{}{
				"md5":     k.MD5,
				"status":  stat,
				"format":  k.Format,
				"quality": k.Quality,
			})
		}
	}
	return items
}

func MarkFailed(key model.ProcessKey) {
	queueMutex.Lock()
	defer queueMutex.Unlock()
	statusMap[key] = "failed"
}

func MarkDone(key model.ProcessKey) {
	queueMutex.Lock()
	defer queueMutex.Unlock()
	delete(queue, key)
	statusMap[key] = "done"
	processed[key] = true
}

func MarkProcessing(key model.ProcessKey) {
	queueMutex.Lock()
	defer queueMutex.Unlock()
	statusMap[key] = "processing"
}