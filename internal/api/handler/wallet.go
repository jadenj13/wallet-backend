package handler

import (
	"fmt"
	"net/http"
)

type Wallet struct{}

func (wallet *Wallet) Init(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Called /wallet/init")

	w.WriteHeader(http.StatusOK)
}

func (wallet *Wallet) Sign(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Called /wallet/sign")

	w.WriteHeader(http.StatusOK)
}
