contract Random {

    Schedule s = Schedule(0x0000000000000000000000000000000000000000);

    struct Pending {
        bytes32 hashOnion;
        uint validSince;
    }

    mapping(address => bytes32) hashOnion;
    
    mapping(address => Pending) pending;

    function newHashOnion(bytes32 _hashOnion) public {
        pending[msg.sender] = Pending({
            hashOnion: _hashOnion,
            validSince: s.schedule()+2
        });
    }
}
