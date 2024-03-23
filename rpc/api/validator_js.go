// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package api

const Validator_JS = `
web3._extend({
	property: 'validator',
	methods:
	[
		new web3._extend.Method({
			name: 'start',
			call: 'validator_start',
			params: 0,
			inputFormatter: [null]
		}),
		new web3._extend.Method({
			name: 'stop',
			call: 'validator_stop',
			params: 0,
			inputFormatter: [null]
		}),
		new web3._extend.Method({
			name: 'setEtherbase',
			call: 'validator_setEtherbase',
			params: 1,
			inputFormatter: [web3._extend.formatters.formatInputInt],
			outputFormatter: web3._extend.formatters.formatOutputBool
		}),
		new web3._extend.Method({
			name: 'setHashonionFilepath',
			call: 'validator_hashonion',
			params: 1,
			inputFormatter: [null],
			outputFormatter: web3._extend.formatters.formatOutputBool
		}),
		new web3._extend.Method({
			name: 'setGasPrice',
			call: 'validator_setGasPrice',
			params: 1,
			inputFormatter: [web3._extend.utils.fromDecmial]
		})
	],
	properties:
	[
	]
});
`
