package cache

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (c *cache) newHTTPServer(addr string) {
	r := chi.NewRouter()
	r.Delete("/{groupName}/{key}", c.deleteHandler)

	// use debug
	r.Get("/{groupName}", c.getGroupHandler)
	r.Get("/{groupName}/{key}", c.getHandler)

	c.httpServ = &http.Server{
		Addr:    addr,
		Handler: r,
	}
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"error": "%s"}`, message)
}

func (c *cache) deleteHandler(w http.ResponseWriter, r *http.Request) {
	groupName := chi.URLParam(r, "groupName")
	key := chi.URLParam(r, "key")
	if groupName == "" || key == "" {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("missing group name(%s) or key(%s)", groupName, key))
		return
	}

	g, err := c.getGroupByName(groupName)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	g.mtx.Lock()
	delete(g.data, key)
	g.mtx.Unlock()

	w.WriteHeader(http.StatusOK)
	w.Write(fmt.Appendf(nil, "key '%s' deleted successfully from group '%s'", key, groupName))
}

func (c *cache) getGroupHandler(w http.ResponseWriter, r *http.Request) {
	groupName := chi.URLParam(r, "groupName")

	if groupName == "" {
		http.Error(w, "missing group name", http.StatusBadRequest)
		return
	}

	g := c.GetGroup(groupName)
	if g == nil {
		http.Error(w, fmt.Sprintf("not found group name '%s'", groupName), http.StatusNotFound)
		return
	}

	dat, err := g.(*group).JSONMarshalIndent("", " ")
	if err != nil {
		http.Error(w, fmt.Sprintf("data marshal failed. err=%v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(dat)

}

func (c *cache) getHandler(w http.ResponseWriter, r *http.Request) {
	groupName := chi.URLParam(r, "groupName")
	key := chi.URLParam(r, "key")

	if groupName == "" || key == "" {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("missing group name(%s) or key(%s)", groupName, key))
		return
	}

	g, err := c.getGroupByName(groupName)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	val, err := g.Get(context.Background(), key)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, fmt.Sprintf("cache miss. key '%s' in group name '%s'", key, groupName))
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("%v", val)))
}
