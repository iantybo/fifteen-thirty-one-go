//go:build windows

// Command cribcounterdll exposes the cribbage counter as a Windows DLL.
//
// Build:
//
//	CGO_ENABLED=1 go build -buildmode=c-shared -o cribcounter.dll ./pkg/cribcounter/dll
//
// This produces cribcounter.dll and a matching cribcounter.h header describing
// the exported C ABI below. Every function that returns a char* returns a
// heap-allocated, NUL-terminated UTF-8 JSON string; the caller MUST release it
// with CribFree to avoid leaking memory.
package main

/*
#include <stdlib.h>
*/
import "C"

import (
	"encoding/json"
	"unsafe"

	"fifteen-thirty-one-go/backend/internal/game/common"
	"fifteen-thirty-one-go/backend/pkg/cribcounter"
)

func main() {}

func jsonResult(v any) *C.char {
	b, err := json.Marshal(v)
	if err != nil {
		return C.CString(`{"error":"failed to encode result"}`)
	}
	return C.CString(string(b))
}

func jsonError(err error) *C.char {
	return jsonResult(map[string]string{"error": err.Error()})
}

// CribCountHand scores a four-card cribbage hand plus a cut card.
//
// hand is a space/comma separated list of exactly four cards (e.g. "5H 5S 5D JC"),
// cut is a single card (e.g. "5C"), and isCrib is non-zero when scoring the crib.
// The return value is a JSON object matching cribcounter.HandScore, or
// {"error": "..."} on failure. Free the result with CribFree.
//
//export CribCountHand
func CribCountHand(hand *C.char, cut *C.char, isCrib C.int) *C.char {
	score, err := cribcounter.CountHandStrings(C.GoString(hand), C.GoString(cut), isCrib != 0)
	if err != nil {
		return jsonError(err)
	}
	return jsonResult(score)
}

// CribCountPegging scores playing a single card during pegging.
//
// seq is a space/comma separated list of the cards already played since the last
// reset (oldest first, may be empty), card is the card being played, and
// currentTotal is the running count before the card is played. The return value
// is a JSON object matching cribcounter.PeggingResult, or {"error": "..."} on
// failure. Free the result with CribFree.
//
//export CribCountPegging
func CribCountPegging(seq *C.char, card *C.char, currentTotal C.int) *C.char {
	playSeq, err := cribcounter.ParseCards(C.GoString(seq))
	if err != nil {
		return jsonError(err)
	}
	newCard, err := common.ParseCard(C.GoString(card))
	if err != nil {
		return jsonError(err)
	}
	result, err := cribcounter.CountPegging(playSeq, newCard, int(currentTotal))
	if err != nil {
		return jsonError(err)
	}
	return jsonResult(result)
}

// CribFree releases a string previously returned by this library.
//
//export CribFree
func CribFree(p *C.char) {
	C.free(unsafe.Pointer(p))
}
