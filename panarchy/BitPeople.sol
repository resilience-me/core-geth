contract BitPeople {

    Schedule schedule = Schedule(0x0000000000000000000000000000000000000000);

    uint entropy;
    function getRandomNumber() internal returns (uint) { return entropy = uint(keccak256(abi.encode(blockhash(block.number - 1), entropy))); }

    struct Nym { uint id; bool verified; }
    struct Pair { bool[2] verified; bool disputed; }
    struct Court { uint id; bool[2] verified; }

    mapping (uint => mapping (address => Nym)) public nym;
    mapping (uint => address[]) public registry;
    mapping (uint => mapping (uint => Pair)) public pair;
    mapping (uint => mapping (address => Court)) public court;
    mapping (uint => uint) public population;
    mapping (uint => mapping (address => bool)) public proofOfUniqueHuman;
    mapping (uint => uint) permits;
    mapping(uint => mapping (uint => uint)) target;
    mapping (uint => uint) traverser;

    enum Token { ProofOfUniqueHuman, Nym, Permit, Border }

    mapping (uint => mapping (Token => mapping (address => uint))) public balanceOf;
    mapping (uint => mapping (Token => mapping (address => mapping (address => uint)))) public allowed;
    
    function registered(uint _t) public view returns (uint) { return registry[_t].length; }
    function getPair(uint _id) public pure returns (uint) { return (_id+1)/2; }
    function getCourt(uint _t, uint _id) public view returns (uint) { if(_id != 0) return 1+(_id-1)%(registered(_t)/2); return 0; }
    function pairVerified(uint _t, uint _id) public view returns (bool) { return (pair[_t][_id].verified[0] == true && pair[_t][_id].verified[1] == true); }
    function deductToken(uint _t, Token _token) internal { require(balanceOf[_t][_token][msg.sender] >= 1); balanceOf[_t][_token][msg.sender]--; }

    function register() external {
        uint t = schedule.schedule();
        deductToken(t, Token.Nym);
        registry[t].push();
        uint counter = registered(t);
        uint id = 1 + getRandomNumber()%counter;
        registry[t][counter-1] = registry[t][id-1];
        registry[t][id-1] = msg.sender;
        nym[t][registry[t][counter-1]].id = counter;
        nym[t][msg.sender].id = id;
    }
    function optIn() external {
        uint t = schedule.schedule();
        deductToken(t, Token.Permit);
        court[t][msg.sender].id = getRandomNumber();
    }
    function verify() external {
        uint t = schedule.schedule()-1;
        require(block.timestamp > schedule.pseudonymEvent(t+1));
        uint id = nym[t][msg.sender].id;
        require(id != 0);
        require(pair[t][getPair(id)].disputed == false);
        pair[t][getPair(id)].verified[id%2] = true;
    }
    function judge(address _court) external {
        uint t = schedule.schedule()-1;
        require(block.timestamp > schedule.pseudonymEvent(t+1));
        uint signer = nym[t][msg.sender].id;
        require(signer != 0);
        require(getCourt(t, court[t][_court].id) == getPair(signer));
        court[t][_court].verified[signer%2] = true;
    }

    function allocateTokens(uint _t) internal {
        balanceOf[_t][Token.ProofOfUniqueHuman][msg.sender]++;
        balanceOf[_t][Token.Nym][msg.sender]++;
        balanceOf[_t][Token.Border][msg.sender]++;
    }
    function nymVerified() external {
        uint t = schedule.schedule()-1;
        require(nym[t][msg.sender].verified == false);
        uint id = nym[t][msg.sender].id;
        require(pairVerified(t, getPair(id)));
        allocateTokens(t+1);
        if(id <= permits[t]) balanceOf[t+1][Token.Permit][msg.sender]++;
        nym[t][msg.sender].verified = true;
    }
    function courtVerified() external {
        uint t = schedule.schedule()-1;
        require(pairVerified(t, getCourt(t, court[t][msg.sender].id)));
        require(court[t][msg.sender].verified[0] == true && court[t][msg.sender].verified[1]
        == true); allocateTokens(t+1);
        delete court[t][msg.sender];
    }
    function claimProofOfUniqueHuman() external {
        uint t = schedule.schedule();
        deductToken(t, Token.ProofOfUniqueHuman);
        proofOfUniqueHuman[t][msg.sender] = true;
        population[t]++;
    }
    function dispute() external {
        uint t = schedule.schedule()-1;
        uint id = getPair(nym[t][msg.sender].id);
        require(id != 0);
        require(!pairVerified(t, id));
        pair[t][id].disputed = true;
    }
    function reassignNym() external {
        uint t = schedule.schedule()-1;
        uint id = nym[t][msg.sender].id;
        require(pair[t][getPair(id)].disputed == true);
        delete nym[t][msg.sender];
        court[t][msg.sender].id = uint(keccak256(abi.encode(id)));
    }
    function reassignCourt() external {
        uint t = schedule.schedule()-1;
        uint id = court[t][msg.sender].id;
        require(pair[t][getCourt(t, id)].disputed == true);
        delete court[t][msg.sender].verified;
        court[t][msg.sender].id = uint(keccak256(abi.encode(id)));
    }
    function borderVote(uint _target) external {
        uint t = schedule.schedule();
        deductToken(t, Token.Border);
        target[t][_target]+=2;
        if(_target > permits[t]) {
            if(traverser[t] < target[t][permits[t]]) traverser[t]++;
            else {
                permits[t]++;
                traverser[t] = 0;
            }
        }
        else if(_target < permits[t]) {
            if(traverser[t] > 0) traverser[t]--;
            else {
                require(permits[t] > 0);
                permits[t]--;
                traverser[t] = target[t][permits[t]-1];
            }
        }
        else traverser[t]++;
    }

    function _transfer(uint _t, address _from, address _to, uint _value, Token _token) internal {
        require(balanceOf[_t][_token][_from] >= _value);
        balanceOf[_t][_token][_from] -= _value;
        balanceOf[_t][_token][_to] += _value;
    }
    function transfer(address _to, uint _value, Token _token) external {
        _transfer(schedule.schedule(), msg.sender, _to, _value, _token);
    }
    function approve(address _spender, uint _value, Token _token) external {
        allowed[schedule.schedule()][_token][msg.sender][_spender] = _value;
    }
    function transferFrom(address _from, address _to, uint _value, Token _token) external {
        uint t = schedule.schedule();
        require(allowed[t][_token][_from][msg.sender] >= _value);
        _transfer(t, _from, _to, _value, _token);
        allowed[t][_token][_from][msg.sender] -= _value;
    }
}
