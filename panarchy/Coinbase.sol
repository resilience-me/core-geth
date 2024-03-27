contract Coinbase is Schedule {

    struct CoinbaseRecord {
        address coinbase;
        uint validSince;
    }
 
    mapping (address => CoinbaseRecord) public coinbase;
    mapping (address => address) public previous;

    function setCoinbase(address _coinbase) public {
        uint t = schedule();
        if (t >= coinbase[msg.sender].validSince) {
            previous[msg.sender] = coinbase[msg.sender].coinbase;
        }
        coinbase[msg.sender].coinbase = _coinbase;
        coinbase[msg.sender].validSince =  t+3;
    }
}
