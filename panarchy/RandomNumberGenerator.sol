contract RandomNumberGenerator {

    Schedule schedule = Schedule(0x0000000000000000000000000000000000000000);
    BitPeople bitpeople = BitPeople(0x0000000000000000000000000000000000000004);

    mapping (uint => mapping (address => bytes32)) commit;
    mapping (uint => uint) votes;

    mapping (uint => mapping (uint => uint)) public points;
    mapping (uint => uint[]) public leaderboard;
    mapping (uint => mapping (uint => uint)) public leaderboardIndex;

    struct Score {
        uint start;
        uint end;
    }
    mapping (uint => mapping (uint => Score)) public segments;

    mapping (uint => mapping (address => bool)) randomToken;

    mapping (uint => mapping (address => uint)) public balanceOf;
    mapping (uint => mapping (address => mapping (address => uint))) public allowed;

    function allocateSuffrageToken() public {
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

        uint score = points[_t][_id];

        if(score == 0) {
            leaderboard[_t].push(_id);
            leaderboardIndex[_t][_id] = leaderboard[_t].length;
            if(segments[_t][1].end == 0) segments[_t][1].end = leaderboard[_t].length;
            segments[_t][1].start = leaderboard[_t].length;
        }
        else {
            uint index = leaderboardIndex[_t][_id];
            uint nextSegment = segments[_t][score].end;
            if(nextSegment != index) {
                leaderboardIndex[_t][_id] = nextSegment;
                leaderboardIndex[_t][leaderboard[_t][nextSegment-1]] = index;
                (leaderboard[_t][nextSegment - 1], leaderboard[_t][index - 1]) = (leaderboard[_t][index - 1], leaderboard[_t][nextSegment - 1]);
            }
            if(segments[_t][score].start == nextSegment) { 
                delete segments[_t][score].start; 
                delete segments[_t][score].end; 
            }
            else segments[_t][score].end++;
            if(segments[_t][score+1].end == 0) segments[_t][score+1].end = nextSegment;
            segments[_t][score+1].start = nextSegment;
        }
        points[_t][_id]++;
    }
}
