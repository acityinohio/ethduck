package bcyeth

import (
	"math/big"
	"time"
)

type Contract struct {
	Solidity       string        `json:"solidity,omitempty"`
	Params         []interface{} `json:"params,omitempty"`
	Publish        []string      `json:"publish,omitempty"`
	Private        string        `json:"private,omitempty"`
	GasLimit       int           `json:"gas_limit,omitempty"`
	Value          big.Int       `json:"value,omitempty"`
	Name           string        `json:"name,omitempty"`
	Bin            string        `json:"bin,omitempty"`
	Address        string        `json:"address,omitempty"`
	Created        time.Time     `json:"created,omitempty"`
	CreationTXHash string        `json:"creation_tx_hash,omitempty"`
	Results        []interface{} `json:"results,omitempty"`
}

func (api *API) CreateContract(contract Contract) (result []Contract, err error) {
	u, err := api.buildURL("/contracts", nil)
	if err != nil {
		return
	}
	err = postResponse(u, &contract, &result)
	return
}

func (api *API) GetContract(address string) (result Contract, err error) {
	u, err := api.buildURL("/contracts/"+address, nil)
	if err != nil {
		return
	}
	err = getResponse(u, &result)
	return
}

func (api *API) CallContract(contract Contract, address string, method string) (result Contract, err error) {
	u, err := api.buildURL("/contracts/"+address+"/"+method, nil)
	if err != nil {
		return
	}
	err = postResponse(u, &contract, &result)
	return
}
