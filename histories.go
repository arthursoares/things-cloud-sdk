package thingscloud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"
	"unicode/utf8"
)

// History represents a synchronization stream. It's identified with a uuid v4
type History struct {
	ID                     string
	Client                 *Client
	LatestServerIndex      int
	LoadedServerIndex      int
	LatestSchemaVersion    int
	EndTotalContentSize    int
	LatestTotalContentSize int
}

type historyResponse struct {
	LatestSchemaVersion    int  `json:"latest-schema-version"`
	LatestTotalContentSize int  `json:"latest-total-content-size"`
	IsEmpty                bool `json:"is-empty"`
	LatestServerIndex      int  `json:"latest-server-index"`
}

// Sync ensures the history object is able to write to things
func (h *History) Sync() error {
	req, err := http.NewRequest("GET", fmt.Sprintf("/version/1/history/%s/items", h.ID), nil)
	if err != nil {
		return err
	}
	query := req.URL.Query()
	query.Add("start-index", strconv.Itoa(h.LatestServerIndex))
	req.URL.RawQuery = query.Encode()
	resp, err := h.Client.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &HTTPError{StatusCode: resp.StatusCode, Status: resp.Status}
	}

	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var v itemsResponse
	if err := json.Unmarshal(bs, &v); err != nil {
		return fmt.Errorf("decoding history sync response: %w", err)
	}
	h.LatestServerIndex = v.CurrentItemIndex
	h.LatestSchemaVersion = v.SchemaVersion
	h.LatestTotalContentSize = v.LatestTotalContentSize
	return nil
}

// History requests a specific history
func (c *Client) History(id string) (*History, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("/version/1/history/%s", id), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, ErrUnauthorized
		}
		return nil, &HTTPError{StatusCode: resp.StatusCode, Status: resp.Status}
	}
	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	h := historyResponse{}
	if err := json.Unmarshal(bs, &h); err != nil {
		return nil, err
	}

	return &History{
		Client:                 c,
		ID:                     id,
		LatestServerIndex:      h.LatestServerIndex,
		LatestSchemaVersion:    h.LatestSchemaVersion,
		LatestTotalContentSize: h.LatestTotalContentSize,
	}, nil
}

// OwnHistory returns the clients own history
func (c *Client) OwnHistory() (*History, error) {
	resp, err := c.Verify()
	if err != nil {
		return nil, err
	}

	return &History{
		Client: c,
		ID:     resp.HistoryKey,
	}, nil
}

// HistoryWithID creates a History object with the given ID without making a network call.
// Use this when you already know the history ID (e.g., from a previous sync).
func (c *Client) HistoryWithID(id string) *History {
	return &History{
		Client: c,
		ID:     id,
	}
}

// Histories requests all known history keys
func (c *Client) Histories() ([]*History, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("/version/1/account/%s/own-history-keys", c.EMail), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Password %s", c.password))
	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, ErrUnauthorized
		}
		return nil, fmt.Errorf("http response code: %s", resp.Status)
	}
	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var keys []string
	if err := json.Unmarshal(bs, &keys); err != nil {
		return nil, fmt.Errorf("decoding history keys: %w", err)
	}

	var histories = make([]*History, len(keys))
	for i, key := range keys {
		histories[i] = &History{
			Client: c,
			ID:     key,
		}
	}
	return histories, nil
}

type createHistoryResponse struct {
	Key string `json:"new-history-key"`
}

// CreateHistory requests a new history key
func (c *Client) CreateHistory() (*History, error) {
	req, err := http.NewRequest("POST", fmt.Sprintf("/version/1/account/%s/own-history-keys", c.EMail), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Password %s", c.password))
	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, ErrUnauthorized
		}
		return nil, fmt.Errorf("http response code: %s", resp.Status)

	}
	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var v createHistoryResponse
	if err := json.Unmarshal(bs, &v); err != nil {
		return nil, fmt.Errorf("decoding create-history response: %w", err)
	}
	return &History{
		Client: c,
		ID:     v.Key,
	}, nil
}

// Delete destroys a history
// Note that thingscloud will always return 202, even if the key is unknown
func (h *History) Delete() error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("/version/1/account/%s/own-history-keys/%s", h.Client.EMail, h.ID), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Password %s", h.Client.password))
	resp, err := h.Client.do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("http response code: %s", resp.Status)
	}
	return nil
}

type commitResponse struct {
	ServerHeadIndex int `json:"server-head-index"`
}

// Identifiable abstracts different thingscloud write requests. As we need to provide a map
// indexed by UUID, all we care about is the ID of the change, not the change itself
type Identifiable interface {
	UUID() string
}

func (h *History) Write(items ...Identifiable) error {
	m := map[string]interface{}{}
	for _, item := range items {
		// A non-canonical identifier permanently corrupts the sync
		// history: Things.app crashes decoding it and the item cannot
		// be removed. Refuse it before anything reaches the server.
		if err := ValidateUUID(item.UUID()); err != nil {
			return fmt.Errorf("refusing to write item: %w", err)
		}
		// The commit body is a map keyed by UUID, so a second op on the
		// same item would silently replace the first. Reject instead.
		if _, dup := m[item.UUID()]; dup {
			return fmt.Errorf("refusing to write items: duplicate UUID %s in one commit — split into separate Write calls", item.UUID())
		}
		m[item.UUID()] = item
	}
	bs, err := json.Marshal(m)
	if err != nil {
		return err
	}
	plan, err := validateTaskWrites(bs)
	if err != nil {
		return err
	}
	ancestorIndex := h.LatestServerIndex
	if plan.needsPreflight() {
		checkedHead, err := h.preflightTask7Modifications(plan.modificationTargets)
		if err != nil {
			return err
		}
		// Use the fixed raw-history snapshot head as the commit ancestor. The
		// method deliberately does not retry if the server rejects the write.
		h.LatestServerIndex = checkedHead
		ancestorIndex = checkedHead
	}
	req, err := http.NewRequest("POST", fmt.Sprintf("/version/1/history/%s/commit", h.ID), bytes.NewReader(bs))
	if err != nil {
		return err
	}
	req.Header.Add("Schema", "301")
	req.Header.Add("Push-Priority", "5")
	// Full App-Instance-Id matching Things format: {hash}-{bundleId}-{hash}
	req.Header.Add("App-Instance-Id", "000000000000000000000000000000000000000000000000000000000000000-com.culturedcode.ThingsMac-000000000000000000000000000000000000000000000000000000000000000")
	req.Header.Add("App-Id", "com.culturedcode.ThingsMac")
	query := req.URL.Query()
	query.Add("ancestor-index", strconv.Itoa(ancestorIndex))
	query.Add("_cnt", "1")
	req.URL.RawQuery = query.Encode()
	resp, err := h.Client.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bs, _ := httputil.DumpResponse(resp, true)
		log.Println(string(bs))
		return fmt.Errorf("Write failed: %d", resp.StatusCode)
	}
	rs, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var w commitResponse
	if err := json.Unmarshal(rs, &w); err != nil {
		return fmt.Errorf("decoding commit response: %w", err)
	}
	h.LatestServerIndex = w.ServerHeadIndex
	return nil
}

type task7TargetState struct {
	exists             bool
	ambiguous          bool
	future             bool
	lastAffectedIndex  int
	sameIndexAmbiguous bool

	rrKnown  bool
	rrActive bool
	rpKnown  bool
	rpActive bool
	rtKnown  bool
	rtActive bool
	icsd     bool
	acrd     bool
}

func (h *History) preflightTask7Modifications(targets map[string]struct{}) (int, error) {
	states := make(map[string]*task7TargetState, len(targets))
	for id := range targets {
		states[id] = &task7TargetState{lastAffectedIndex: -1}
	}

	// Items mutates its receiver. Scan through a copy so every failed or
	// incomplete preflight leaves the caller's History untouched.
	snapshot := *h
	snapshot.LatestServerIndex = 0
	snapshot.LoadedServerIndex = 0
	start, checkedHead := 0, -1
	for {
		items, _, err := snapshot.Items(ItemsOptions{StartIndex: start})
		if err != nil {
			return 0, fmt.Errorf("Task7 write preflight: %w", err)
		}
		if checkedHead < 0 {
			checkedHead = snapshot.LatestServerIndex
			if checkedHead < 0 {
				return 0, fmt.Errorf("Task7 write preflight: invalid history head")
			}
		} else if snapshot.LatestServerIndex < checkedHead {
			return 0, fmt.Errorf("Task7 write preflight: history head regressed")
		}

		for _, item := range items {
			if !item.HasServerIndex || item.ServerIndex >= checkedHead {
				continue
			}
			if err := applyTask7PreflightItem(states, item); err != nil {
				return 0, err
			}
		}
		if snapshot.LoadedServerIndex >= checkedHead {
			break
		}
		if snapshot.LoadedServerIndex <= start {
			return 0, fmt.Errorf("Task7 write preflight: history ended before checked head %d", checkedHead)
		}
		start = snapshot.LoadedServerIndex
	}

	for id, state := range states {
		switch {
		case state.future:
			return 0, fmt.Errorf("Task7 write preflight: target %s has an unsupported future task kind", id)
		case !state.exists || state.ambiguous || state.sameIndexAmbiguous:
			return 0, fmt.Errorf("Task7 write preflight: target %s is missing or ambiguous", id)
		case !state.rrKnown || !state.rpKnown || !state.rtKnown:
			return 0, fmt.Errorf("Task7 write preflight: target %s lacks complete recurrence markers", id)
		case state.rrActive || state.rpActive || state.rtActive || state.icsd || state.acrd:
			return 0, fmt.Errorf("Task7 write preflight: target %s is recurring", id)
		}
	}
	return checkedHead, nil
}

func applyTask7PreflightItem(states map[string]*task7TargetState, item Item) error {
	if item.Kind == ItemKind("Tombstone2") || item.Kind == ItemKind("Tombstone") {
		if !utf8.Valid(item.P) || rejectUnpairedJSONSurrogates(item.P) != nil || rejectDuplicateJSONKeys(item.P, true) != nil {
			return fmt.Errorf("Task7 write preflight: malformed tombstone in raw history")
		}
		var payload TombstoneActionItemPayload
		if err := json.Unmarshal(item.P, &payload); err != nil {
			return fmt.Errorf("Task7 write preflight: malformed tombstone in raw history")
		}
		fields, err := jsonObject(item.P)
		if err != nil {
			return fmt.Errorf("Task7 write preflight: malformed tombstone in raw history")
		}
		if _, ok := jsonString(fields["dloid"]); !ok || payload.DeletedObjectID == "" {
			return fmt.Errorf("Task7 write preflight: malformed tombstone in raw history")
		}
		target := payload.DeletedObjectID
		if item.Kind == ItemKind("Tombstone") && ValidateUUID(target) != nil {
			target = EncodeLegacyIdentifier(target)
		}
		if state := states[target]; state != nil {
			state.markAffected(item.ServerIndex)
			state.resetSemantic(false)
		}
		return nil
	}

	id := item.UUID
	if isLegacyTaskKind(item.Kind) && ValidateUUID(id) != nil {
		id = EncodeLegacyIdentifier(id)
	}
	state := states[id]
	if state == nil {
		return nil
	}
	state.markAffected(item.ServerIndex)

	switch string(item.Kind) {
	case "Task6", "Task7", "Task4", "Task3", "Task":
	case "Tombstone2", "Tombstone":
		return nil
	default:
		if strings.HasPrefix(string(item.Kind), "Task") {
			state.future = true
		}
		state.ambiguous = true
		return nil
	}

	switch item.Action {
	case ItemActionCreated:
		ambiguous := state.ambiguous || state.future || state.exists
		state.resetSemantic(true)
		state.ambiguous = ambiguous
	case ItemActionModified:
		if !state.exists {
			state.ambiguous = true
		}
	case ItemActionDeleted:
		state.resetSemantic(false)
		return nil
	default:
		state.ambiguous = true
		return nil
	}
	if !utf8.Valid(item.P) || rejectUnpairedJSONSurrogates(item.P) != nil || rejectDuplicateJSONKeys(item.P, true) != nil {
		state.ambiguous = true
		return nil
	}
	payload, err := jsonObject(item.P)
	if err != nil {
		state.ambiguous = true
		return nil
	}
	applyRawRecurrenceMarker(payload, "rr", &state.rrKnown, &state.rrActive)
	applyRawRecurrenceMarker(payload, "rp", &state.rpKnown, &state.rpActive)
	if raw, ok := payload["rt"]; ok {
		state.rtKnown = true
		if jsonNull(raw) {
			state.rtActive = false
		} else if values, err := jsonStringArray(raw); err == nil {
			state.rtActive = len(values) != 0
		} else {
			state.ambiguous = true
		}
	}
	applyRawPresenceMarker(payload, "icsd", &state.icsd)
	applyRawPresenceMarker(payload, "acrd", &state.acrd)
	return nil
}

func (s *task7TargetState) markAffected(serverIndex int) {
	if s.lastAffectedIndex == serverIndex {
		s.sameIndexAmbiguous = true
	}
	s.lastAffectedIndex = serverIndex
}

func (s *task7TargetState) resetSemantic(exists bool) {
	lastIndex, sameIndexAmbiguous := s.lastAffectedIndex, s.sameIndexAmbiguous
	*s = task7TargetState{
		exists:             exists,
		lastAffectedIndex:  lastIndex,
		sameIndexAmbiguous: sameIndexAmbiguous,
	}
}

func isLegacyTaskKind(kind ItemKind) bool {
	switch string(kind) {
	case "Task4", "Task3", "Task":
		return true
	default:
		return false
	}
}

func applyRawRecurrenceMarker(payload map[string]json.RawMessage, key string, known, active *bool) {
	if raw, ok := payload[key]; ok {
		*known = true
		*active = !jsonNull(raw)
	}
}

func applyRawPresenceMarker(payload map[string]json.RawMessage, key string, active *bool) {
	if raw, ok := payload[key]; ok {
		*active = !jsonNull(raw)
	}
}
