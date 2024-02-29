package goods

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"hezzl_test/internal/entity"
	resp "hezzl_test/internal/lib/api/response"
	"hezzl_test/internal/storage/postgres"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Goods interface {
	CreateGood(projectId int, name string) (entity.GoodCreateResponse, error)
	UpdateGood(id, projectId int, name, description string) (entity.GoodUpdateResponse, error)
	DeleteGood(id, projectId int) (entity.GoodRemoveResponse, string, string, int, error)
	GetGoodByID(key int) (entity.GoodsForList, error)
	CalculateTotalAndRemoved() (int, int, error)
	Reprioritize(goodID, projectID, newPriority int) (string, string, error)
}

func Create(log *slog.Logger, goods Goods, natsConn *nats.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.goods.Create"

		log := log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		projectId := chi.URLParam(r, "projectId")
		if projectId == "" {
			log.Info("project id is empty")
			w.WriteHeader(http.StatusBadRequest)
			render.JSON(w, r, resp.Error("project id parameter is required"))
			return
		}

		projectIdInt, err := strconv.Atoi(projectId)
		if err != nil {
			http.Error(w, "Invalid project ID", http.StatusBadRequest)
			return
		}

		var req entity.GoodCreateRequest

		err = render.DecodeJSON(r.Body, &req)
		if errors.Is(err, io.EOF) {
			log.Error("request body is empty")

			render.JSON(w, r, resp.Error("empty request"))

			return
		}
		if err != nil {
			log.Error("failed to decode request body: ", err)

			render.JSON(w, r, resp.Error("failed to decode request"))

			return
		}

		log.Info("request body decoded", slog.Any("request", req))

		response, err := goods.CreateGood(projectIdInt, req.Name)
		if err != nil {
			log.Error("failed to create good: ", err)

			render.JSON(w, r, resp.Error("internal error"))

			return
		}

		log.Info("good created")

		w.WriteHeader(http.StatusCreated)
		render.JSON(w, r, response)

		event := &entity.GoodEvent{
			Id:          response.Id,
			ProjectId:   response.ProjectId,
			Name:        response.Name,
			Description: response.Description,
			Priority:    response.Priority,
			Removed:     false,
			EventTime:   response.CreatedAt,
		}

		eventData, err := json.Marshal(event)
		if err != nil {
			log.Error("Error marshaling message: ", err)
			return
		}

		err = natsConn.Publish("goods.created", eventData)
		if err != nil {
			log.Error("Error sending message to NATS: ", err)
			return
		}

		log.Info("message sended to NATS")
	}
}

func Update(log *slog.Logger, goods Goods, redisClient *redis.Client, natsConn *nats.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.goods.Update"

		log := log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		projectId := chi.URLParam(r, "projectId")
		if projectId == "" {
			log.Info("project id is empty")
			w.WriteHeader(http.StatusBadRequest)
			render.JSON(w, r, resp.Error("project id parameter is required"))
			return
		}

		id := chi.URLParam(r, "id")
		if projectId == "" {
			log.Info("id is empty")
			w.WriteHeader(http.StatusBadRequest)
			render.JSON(w, r, resp.Error("id parameter is required"))
			return
		}

		projectIdInt, err := strconv.Atoi(projectId)
		if err != nil {
			http.Error(w, "Invalid project ID", http.StatusBadRequest)
			return
		}

		idInt, err := strconv.Atoi(id)
		if err != nil {
			http.Error(w, "Invalid ID", http.StatusBadRequest)
			return
		}

		var req entity.GoodUpdateRequest

		err = render.DecodeJSON(r.Body, &req)
		if errors.Is(err, io.EOF) {
			log.Error("request body is empty")

			render.JSON(w, r, resp.Error("empty request"))

			return
		}
		if err != nil {
			log.Error("failed to decode request body: ", err)

			render.JSON(w, r, resp.Error("failed to decode request"))

			return
		}

		log.Info("request body decoded", slog.Any("request", req))

		if req.Name == "" {
			log.Error("failed to update good: name is cant be empty")

			w.WriteHeader(http.StatusBadRequest)
			render.JSON(w, r, resp.Error("name is cant be empty"))

			return
		}

		response, err := goods.UpdateGood(idInt, projectIdInt, req.Name, req.Description)
		if err != nil {
			if err == postgres.ErrNotFound {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"code":    3,
					"message": "errors.good.notFound",
					"details": map[string]string{},
				})
				return
			}
			log.Error("failed to update good: ", err)

			render.JSON(w, r, resp.Error("internal error"))

			return
		}

		log.Info("good updated")

		w.WriteHeader(http.StatusOK)
		render.JSON(w, r, response)

		event := &entity.GoodEvent{
			Id:          response.Id,
			ProjectId:   response.ProjectId,
			Name:        response.Name,
			Description: response.Description,
			Priority:    response.Priority,
			Removed:     false,
			EventTime:   time.Now(),
		}

		eventData, err := json.Marshal(event)
		if err != nil {
			log.Error("Error marshaling message: ", err)
			return
		}

		err = natsConn.Publish("goods.updated", eventData)
		if err != nil {
			log.Error("Error sending message to NATS: ", err)
			return
		}

		log.Info("message sended to NATS")

		err = InvalidateRedisCache(redisClient, idInt)
		if err != nil {
			log.Error("Redis cache invalidation error: ", err)
		}

		log.Info("good deleted from REDIS")
	}
}

func Remove(log *slog.Logger, goods Goods, redisClient *redis.Client, natsConn *nats.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.goods.Remove"

		log := log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		projectId := chi.URLParam(r, "projectId")
		if projectId == "" {
			log.Info("project id is empty")
			w.WriteHeader(http.StatusBadRequest)
			render.JSON(w, r, resp.Error("project id parameter is required"))
			return
		}

		id := chi.URLParam(r, "id")
		if id == "" {
			log.Info("id is empty")
			w.WriteHeader(http.StatusBadRequest)
			render.JSON(w, r, resp.Error("id parameter is required"))
			return
		}

		projectIdInt, err := strconv.Atoi(projectId)
		if err != nil {
			http.Error(w, "invalid project ID", http.StatusBadRequest)
			return
		}

		idInt, err := strconv.Atoi(id)
		if err != nil {
			http.Error(w, "invalid ID", http.StatusBadRequest)
			return
		}

		response, name, description, priority, err := goods.DeleteGood(idInt, projectIdInt)
		if err != nil {
			if err == postgres.ErrNotFound {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"code":    3,
					"message": "errors.good.notFound",
					"details": map[string]string{},
				})
				return
			}
			log.Error("failed to remove good: ", err)

			render.JSON(w, r, resp.Error("internal error"))

			return
		}

		log.Info("good removed")

		w.WriteHeader(http.StatusOK)
		render.JSON(w, r, response)

		event := &entity.GoodEvent{
			Id:          response.Id,
			ProjectId:   response.ProjectId,
			Name:        name,
			Description: description,
			Priority:    priority,
			Removed:     true,
			EventTime:   time.Now(),
		}

		eventData, err := json.Marshal(event)
		if err != nil {
			log.Error("Error marshaling message: ", err)
			return
		}

		err = natsConn.Publish("goods.removed", eventData)
		if err != nil {
			log.Error("Error sending message to NATS: ", err)
			return
		}

		log.Info("message sended to NATS")

		err = InvalidateRedisCache(redisClient, idInt)
		if err != nil {
			log.Error("Redis cache invalidation error: ", err)
		}

		log.Info("good deleted from REDIS")
	}
}

func List(log *slog.Logger, goods Goods, redisClient *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.goods.List"

		log := log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		var (
			limitInt  int
			offsetInt int
		)

		limit := r.URL.Query().Get("limit")
		if limit == "" {
			limitInt = 10
			log.Info("limit is set to 10")
		} else {
			limitInt, _ = strconv.Atoi(limit)
		}

		offset := r.URL.Query().Get("offset")
		if offset == "" {
			offsetInt = 1
			log.Info("offset is set to 1")
		} else {
			offsetInt, _ = strconv.Atoi(offset)
		}

		keys := make([]string, limitInt)
		for i := 0; i < limitInt; i++ {
			keys[i] = fmt.Sprintf("goods:%d", offsetInt+i)
		}

		ctx := context.Background()
		goodsList := make([]entity.GoodsForList, 0)

		for _, key := range keys {
			result, err := redisClient.Get(ctx, key).Result()
			if err == redis.Nil {
				parts := strings.Split(key, ":")
				if len(parts) != 2 {
					log.Error("incorrect format of the redis key")
					continue
				}
				keyInt, err := strconv.Atoi(parts[1])
				if err != nil {
					log.Error("incorrect good id: ", err)
					continue
				}
				good, err := goods.GetGoodByID(keyInt)
				if err != nil {
					log.Error("error fetching good by ID: "+key, err)
					continue
				}
				jsonData, _ := json.Marshal(good)
				redisClient.Set(ctx, key, jsonData, time.Minute)
				goodsList = append(goodsList, good)
			} else if err != nil {
				log.Error("error fetching from Redis: ", err)
				continue
			} else {
				var good entity.GoodsForList
				_ = json.Unmarshal([]byte(result), &good)
				goodsList = append(goodsList, good)
			}
		}

		if len(goodsList) == 0 {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]interface{}{})
			return
		}

		total, removed, err := goods.CalculateTotalAndRemoved()
		if err != nil {
			log.Error("error : ", err)
		}

		response := entity.GoodsListResponse{
			Meta: entity.MetaForList{
				Total:   total,
				Removed: removed,
				Limit:   limitInt,
				Offset:  offsetInt,
			},
			Goods: goodsList,
		}

		log.Info("list geted")

		w.WriteHeader(http.StatusOK)
		render.JSON(w, r, response)
	}
}

func Reprioritize(log *slog.Logger, goods Goods, redisClient *redis.Client, natsConn *nats.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.goods.Reprioritize"

		log := log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		projectId := chi.URLParam(r, "projectId")
		if projectId == "" {
			log.Info("project id is empty")
			w.WriteHeader(http.StatusBadRequest)
			render.JSON(w, r, resp.Error("project id parameter is required"))
			return
		}

		id := chi.URLParam(r, "id")
		if projectId == "" {
			log.Info("id is empty")
			w.WriteHeader(http.StatusBadRequest)
			render.JSON(w, r, resp.Error("id parameter is required"))
			return
		}

		projectIdInt, err := strconv.Atoi(projectId)
		if err != nil {
			http.Error(w, "Invalid project ID", http.StatusBadRequest)
			return
		}

		idInt, err := strconv.Atoi(id)
		if err != nil {
			http.Error(w, "Invalid ID", http.StatusBadRequest)
			return
		}

		var req entity.ReprioritizeRequest

		err = render.DecodeJSON(r.Body, &req)
		if errors.Is(err, io.EOF) {
			log.Error("request body is empty")

			render.JSON(w, r, resp.Error("empty request"))

			return
		}
		if err != nil {
			log.Error("failed to decode request body: ", err)

			render.JSON(w, r, resp.Error("failed to decode request"))

			return
		}

		log.Info("request body decoded", slog.Any("request", req))

		name, description, err := goods.Reprioritize(idInt, projectIdInt, req.NewPriority)
		if err != nil {
			if err == postgres.ErrNotFound {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"code":    3,
					"message": "errors.good.notFound",
					"details": map[string]string{},
				})
				return
			}
			log.Error("Error updating priorities: ", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		event := &entity.GoodEvent{
			Id:          idInt,
			ProjectId:   projectIdInt,
			Name:        name,
			Description: description,
			Priority:    req.NewPriority,
			Removed:     true,
			EventTime:   time.Now(),
		}

		eventData, err := json.Marshal(event)
		if err != nil {
			log.Error("Error marshaling message: ", err)
			return
		}

		err = natsConn.Publish("goods.removed", eventData)
		if err != nil {
			log.Error("Error sending message to NATS: ", err)
			return
		}

		log.Info("message sended to NATS")

		response := entity.ReprioritizeResponse{
			Id:       idInt,
			Priority: req.NewPriority,
		}

		w.WriteHeader(http.StatusOK)
		render.JSON(w, r, response)

		err = InvalidateRedisCache(redisClient, idInt)
		if err != nil {
			log.Error("Redis cache invalidation error: ", err)
		}

		log.Info("good deleted from REDIS")
	}
}

func InvalidateRedisCache(redisClient *redis.Client, goodID int) error {
	const op = "handlers.goods.InvalidateRedisCache"

	ctx := context.Background()
	key := fmt.Sprintf("goods:%d", goodID)
	_, err := redisClient.Del(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("failed to invalidate redis cache for good ID %s:%d: %w", op, goodID, err)
	}
	return nil
}
