contract RandomNumberGenerator {

    Schedule schedule = Schedule(0x0000000000000000000000000000000000000000);
    BitPeople bitpeople = BitPeople(0x0000000000000000000000000000000000000004);

    mapping (uint => mapping (address => bytes32)) commit;
    mapping (uint => uint) votes;
    mapping (uint => mapping (uint => uint)) points;
    mapping (uint => uint) highscore;
    mapping (uint => uint) public winner;
    mapping (uint => mapping (address => bool)) claimed;
    mapping (uint => mapping (address => uint)) public balanceOf;
    mapping (uint => mapping (address => mapping (address => uint))) public allowed;

    function commitHash(bytes32 _hash) public {
        uint t = schedule.schedule();
        uint deadline = schedule.toSeconds(t)+schedule.period()/2;
        require(block.timestamp < deadline && balanceOf[t][msg.sender] >= 1);
        balanceOf[t][msg.sender]--;
        commit[t][msg.sender] = _hash;
        votes[t]++;
    }
    function revealHash(bytes32 _preimage) public {
        uint t = schedule.schedule();
        uint start = schedule.toSeconds(t)+schedule.period()/2;
        require(block.timestamp >= start && keccak256(abi.encode(_preimage)) == commit[t-1][msg.sender]);
        uint id = winner[t-1];
        address seed = bitpeople.registry[t-1][id%bitpeople.registry[t-1].length];
        bytes32 mutated = keccak256(abi.encode(_preimage, seed));
        uint id = uint(mutated)%votes[t];
        points[_t][_id]++;
        if (points[_t][_id] > highscore[_t]) {
            highscore[_t]++;
            winner[_t] = _id;
        }
        delete commit[t-1][msg.sender];
    }
    function allocateRandomToken() public {
        uint t = schedule.schedule();
        require(bitpeople.proofOfUniqueHuman(t, msg.sender) && !claimed[t][msg.sender]);
        balanceOf[t][msg.sender]++;
        claimed[t][msg.sender] = true;
    }
}
