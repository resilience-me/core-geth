contract Random is Schedule {

    struct Pending {
        bytes32 hashOnion;
        uint validSince;
    }

    mapping(address => bytes32) hashOnion;
    
    mapping(address => Pending) pending;

    function newHashOnion(bytes32 _hashOnion) public {
        pending[msg.sender] = Pending({
            hashOnion: _hashOnion,
            validSince: schedule()+2
        });
    }
}
