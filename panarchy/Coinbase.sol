contract Coinbase {
    Schedule schedule = Schedule(0x0000000000000000000000000000000000000000);

    struct CoinbaseRecord {
        address coinbase;
        uint validSince;
    }
 
    mapping (address => CoinbaseRecord) public coinbase;
    mapping (address => address) public previous;

    function setCoinbase(address _coinbase) public {
        uint t = schedule.schedule();
        if (t >= coinbase[msg.sender].validSince) {
            previous[msg.sender] = coinbase[msg.sender].coinbase;
        }
        coinbase[msg.sender].coinbase = _coinbase;
        coinbase[msg.sender].validSince =  t+3;
    }
}
