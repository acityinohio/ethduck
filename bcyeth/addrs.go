package bcyeth

import "math/big"

type Addr struct {
	Address            string  `json:"address"`
	TotalReceived      big.Int `json:"total_received"`
	TotalSent          big.Int `json:"total_sent"`
	Balance            big.Int `json:"balance"`
	UnconfirmedBalance big.Int `json:"unconfirmed_balance"`
	FinalBalance       big.Int `json:"final_balance"`
	NTx                int     `json:"n_tx"`
	UnconfirmedNTx     int     `json:"unconfirmed_n_tx"`
	FinalNTx           int     `json:"final_n_tx"`
}

func (api *API) GetAddrBal(addr string) (address Addr, err error) {
	u, err := api.buildURL("/addrs/"+addr+"/balance", nil)
	if err != nil {
		return
	}
	err = getResponse(u, &address)
	return
}
