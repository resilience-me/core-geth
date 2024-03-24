contract RandomNumberGenerator {

    Schedule schedule = Schedule(0x0000000000000000000000000000000000000000);
    BitPeople bitpeople = BitPeople(0x0000000000000000000000000000000000000004);

    mapping (uint => mapping (address => bytes32)) commit;
    mapping (uint => uint) votes;

    mapping (uint => mapping (uint => uint)) points;
    mapping (uint => uint) highscore;
    mapping (uint => uint) public winner;

    mapping (uint => mapping (address => bool)) randomToken;

    mapping (uint => mapping (address => uint)) public balanceOf;
    mapping (uint => mapping (address => mapping (address => uint))) public allowed;

    function allocateRandomToken() public {
        uint t = schedule.schedule();
        require(bitpeople.proofOfUniqueHuman(t, msg.sender));
        require(!randomToken[t][msg.sender]);
        balanceOf[t][msg.sender]++;
        randomToken[t][msg.sender] = true;
    }
    function commitHash(bytes32 _hash) public {
        uint t = schedule.schedule();
        uint deadline = schedule.toSeconds(t)+schedule.period()/2;
        require(block.timestamp < deadline);
        require(balanceOf[t][msg.sender] >= 1);
        balanceOf[t][msg.sender]--;
        commit[t][msg.sender] = _hash;
        votes[t]++;
    }
    function revealHash(bytes32 _preimage) public {
        uint t = schedule.schedule();
        uint start = schedule.toSeconds(t)+schedule.period()/2;
        require(block.timestamp >= start);
        require(keccak256(abi.encode(_preimage)) == commit[t][msg.sender]);
        uint id = uint(_preimage)%votes[t];
        vote(id, t);
        delete commit[t][msg.sender];
    }
    function vote(uint _id, uint _t) internal {
        points[_t][_id]++;
        if (points[_t][_id] > highscore[_t]) {
            highscore[_t]++;
            winner[_t] = _id;
        }
    }
}
