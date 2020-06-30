package incoming

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"gomq/dbs"
	"gomq/packages/inrouting"
	"gomq/utils"
	"gopkg.in/mgo.v2/bson"
	"net/http"
	"time"

	"gitlab.com/quangdangfit/gocommon/utils/logger"
)

const (
	RequestTimeout = 60
)

type Handler interface {
	HandleMessage(message *InMessage, routingKey string) error
	storeMessage(message *InMessage) (err error)
	callAPI(message *InMessage) (*http.Response, error)
}

type handler struct {
	msgRepo     Repository
	routingRepo inrouting.Repository
}

func NewHandler() Handler {
	r := handler{
		msgRepo:     NewRepository(),
		routingRepo: inrouting.NewRepository(),
	}
	return &r
}

func (h *handler) HandleMessage(message *InMessage, routingKey string) error {

	defer h.storeMessage(message)

	query := bson.M{"name": routingKey}
	inRoutingKey, err := h.routingRepo.GetRoutingKey(query)
	if err != nil {
		message.Status = dbs.InMessageStatusInvalid
		message.Logs = append(message.Logs, utils.ParseError(err))
		logger.Error("Cannot find routing key ", err)
		return err
	}
	message.RoutingKey = *inRoutingKey

	prevRoutingKey, _ := h.routingRepo.GetPreviousRoutingKey(message.RoutingKey)
	if prevRoutingKey != nil {
		prevMsg, _ := h.getPreviousMessage(*message, prevRoutingKey.Name)

		if prevMsg == nil || (prevMsg.Status != dbs.InMessageStatusSuccess &&
			prevMsg.Status != dbs.InMessageStatusCanceled) {

			message.Status = dbs.InMessageStatusWaitPrevMsg

			logger.Warn("Set message to WAIT_PREV_MESSAGE")
			return nil
		}
	}

	res, err := h.callAPI(message)
	if err != nil {
		message.Status = dbs.InMessageStatusWaitRetry
		message.Logs = append(message.Logs, utils.ParseError(err))
		return err
	}

	if res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusUnauthorized {
		message.Status = dbs.InMessageStatusWaitRetry
		err = errors.New(fmt.Sprintf("failed to call API %s", res.Status))
		message.Logs = append(message.Logs, utils.ParseError(res.Status))
		return err
	}

	if res.StatusCode != http.StatusOK {
		message.Status = dbs.InMessageStatusWaitRetry
		err = errors.New("failed to call API")
		message.Logs = append(message.Logs, utils.ParseError(res))
		return err
	}

	message.Status = dbs.InMessageStatusSuccess
	message.Logs = append(message.Logs, utils.ParseError(res))

	return nil
}

func (h *handler) storeMessage(message *InMessage) (err error) {
	msg, err := h.msgRepo.GetSingleInMessage(bson.M{"id": message.ID})
	if msg != nil {
		err = h.msgRepo.UpdateInMessage(message)
		if err != nil {
			logger.Errorf("[Handle InMsg] Failed to update msg %s, %s", message.ID, err)
			return err
		}

		logger.Infof("[Handle InMsg] Updated msg %s", message.ID)
		return nil
	}

	err = h.msgRepo.AddInMessage(message)
	if err != nil {
		logger.Errorf("[Handle InMsg] Failed to insert msg %s, %s", message.ID, err)
		return err
	}
	logger.Infof("[Handle InMsg] Inserted msg %s", message.ID)
	return nil
}

func (h *handler) callAPI(message *InMessage) (*http.Response, error) {
	routingKey := message.RoutingKey

	bytesPayload, _ := json.Marshal(message.Payload)
	req, _ := http.NewRequest(
		routingKey.APIMethod, routingKey.APIUrl, bytes.NewBuffer(bytesPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", "ahsfishdi"))
	req.Header.Set("x-api-key", message.APIKey)

	client := http.Client{
		Timeout: RequestTimeout * time.Second,
	}
	res, err := client.Do(req)

	if err != nil {
		logger.Errorf("Failed to send request to %s, %s", routingKey.APIUrl, err)
		return res, err
	}

	return res, nil
}

func (h *handler) getPreviousMessage(message InMessage, routingKey string) (
	*InMessage, error) {

	query := bson.M{
		"origin_model":     message.OriginModel,
		"origin_code":      message.OriginCode,
		"routing_key.name": routingKey,
	}
	return h.msgRepo.GetSingleInMessage(query)
}
