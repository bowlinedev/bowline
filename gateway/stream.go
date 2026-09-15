package gateway

import "net/http"

const streamChunk = 32 << 10

func (g *Gateway) stream(w http.ResponseWriter, req *http.Request, rt route) {
	ctx := req.Context()
	outbound, err := g.request(ctx, req, rt, req.Body)
	if err != nil {
		writeEnvelope(w, "INTERNAL", err.Error())
		return
	}
	resp, err := g.clients[rt.service].Do(outbound)
	if err != nil {
		g.writeContextError(w, ctx, rt)
		return
	}
	defer resp.Body.Close()
	copyResponseHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	controller := http.NewResponseController(w)
	controller.Flush()

	buf := make([]byte, streamChunk)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				return
			}
			controller.Flush()
		}
		if readErr != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}
