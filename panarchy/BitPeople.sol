contract Schedule {

    uint constant public genesis = 1710568800;
    uint constant public period = 4 weeks;

    function schedule() public view returns(uint) { return ((block.timestamp - genesis) / period); }
    function toSeconds(uint _t) public pure returns (uint) { return genesis + _t * period; }
    function quarter(uint _t) public view returns (uint) { return (block.timestamp-toSeconds(_t))/(period/4); }
    function hour(uint _t) public pure returns (uint) { return 1 + uint(keccak256(abi.encode(_t)))%24; }
    function pseudonymEvent(uint _t) public pure returns (uint) { return toSeconds(_t) + hour(_t)*1 hours; }
}

contract Kleroterion {
    mapping (uint => uint) public winner;
}

contract Mixer is Schedule {

    enum Token { ProofOfUniqueHuman, Nym, Permit, Border }
    mapping (uint => mapping (Token => mapping (address => uint))) public balanceOf;
    mapping (uint => mapping (Token => mapping (address => mapping (address => uint)))) public allowed;

    function _transfer(uint _t, address _from, address _to, uint _value, Token _token) internal {
        require(balanceOf[_t][_token][_from] >= _value);
        balanceOf[_t][_token][_from] -= _value;
        balanceOf[_t][_token][_to] += _value;
    }
    function transfer(address _to, uint _value, Token _token) external {
    _transfer(schedule(), msg.sender, _to, _value, _token);
    }
    function approve(address _spender, uint _value, Token _token) external {
        allowed[schedule()][_token][msg.sender][_spender] = _value;
    }
    function transferFrom(address _from, address _to, uint _value, Token _token) external {
        uint t = schedule();
        require(allowed[t][_token][_from][msg.sender] >= _value);
        _transfer(t, _from, _to, _value, _token);
        allowed[t][_token][_from][msg.sender] -= _value;
    }
}

contract BitPeople is Mixer {

    Kleroterion kleroterion = Kleroterion(0x0000000000000000000000000000000000000004);

    bytes32 random;

    struct Nym { uint id; bool verified; }
    struct Pair { bool[2] verified; bool disputed; }
    struct Court { uint id; bool[2] verified; }

    mapping (uint => mapping (address => Nym)) nym;
    mapping (uint => address[]) public registry;
    mapping (uint => uint) shuffled;
    mapping (uint => mapping (uint => Pair)) public pair;
    mapping (uint => mapping (address => Court)) public court;
    mapping (uint => uint) courts;
    mapping (uint => uint) public population;
    mapping (uint => mapping (address => bool)) public proofOfUniqueHuman;
    mapping (uint => uint) permits;
    mapping(uint => mapping (uint => uint)) target;
    mapping (uint => uint) traverser;

    function registered(uint _t) public view returns (uint) { return registry[_t].length; }
    function getPair(uint _id) public pure returns (uint) { return (_id+1)/2; }
    function getCourt(uint _t, uint _id) public view returns (uint) { if(_id != 0) return 1+(_id-1)%(registered(_t)/2); return 0; }
    function pairVerified(uint _t, uint _id) public view returns (bool) { return (pair[_t][_id].verified[0] == true && pair[_t][_id].verified[1] == true); }
    function deductToken(uint _t, Token _token) internal { require(balanceOf[_t][_token][msg.sender] >= 1); balanceOf[_t][_token][msg.sender]--; }

    function register() external {
        uint t = schedule();
        require(quarter(t) < 2);
        deductToken(t, Token.Nym);
        registry[t].push(msg.sender);
    }
    function optIn() external {
        uint t = schedule();
        require(quarter(t) < 2);
        deductToken(t, Token.Permit);
        courts[t]++;
        court[t][msg.sender].id = courts[t];
    }
    function _shuffle(uint _t) internal returns (bool) {
        uint _shuffled = shuffled[_t];
        if(_shuffled == 0) random = keccak256(abi.encode(random, kleroterion.winner(_t)));
        uint unshuffled = registered(_t)-_shuffled;
        if(unshuffled == 0) return false;
        uint index = _shuffled + uint(random)%unshuffled;
        address randomNym = registry[_t][index];
        registry[_t][index] = registry[_t][_shuffled];
        registry[_t][_shuffled] = randomNym;
        nym[_t][randomNym].id = _shuffled+1;
        shuffled[_t]++;
        random = keccak256(abi.encode(random, randomNym));
        return true;
    }

    function shuffle() external returns (bool)  {
        uint t = schedule();
        require(quarter(t) == 3);
        return _shuffle(t);
    }
    function lateShuffle() external returns (bool) {
        uint t = schedule()-1;
        require(quarter(t) < 2);
        return _shuffle(t);
    }
    function verify() external {
            uint t = schedule()-1;
            require(block.timestamp > pseudonymEvent(t+1));
            uint id = nym[t][msg.sender].id;
            require(id != 0);
            require(pair[t][getPair(id)].disputed == false);
            pair[t][getPair(id)].verified[id%2] = true;
    }
    function judge(address _court) external {
            uint t = schedule()-1;
            require(block.timestamp > pseudonymEvent(t+1));
            uint signer = nym[t][msg.sender].id;
            require(getCourt(t, court[t][_court].id) == getPair(signer));
            court[t][_court].verified[signer%2] = true;
    }

    function allocateTokens(uint _t) internal {
            balanceOf[_t][Token.ProofOfUniqueHuman][msg.sender]++;
            balanceOf[_t][Token.Nym][msg.sender]++;
            balanceOf[_t][Token.Border][msg.sender]++;
    }
    function nymVerified() external {
            uint t = schedule()-1;
            require(nym[t][msg.sender].verified == false);
            uint id = nym[t][msg.sender].id;
            require(pairVerified(t, getPair(id)));
            allocateTokens(t+1);
            if(id <= permits[t]) balanceOf[t+1][Token.Permit][msg.sender]++;
            nym[t][msg.sender].verified = true;
    }
    function courtVerified() external {
            uint t = schedule()-1;
            require(pairVerified(t, getCourt(t, court[t][msg.sender].id)));
            require(court[t][msg.sender].verified[0] == true && court[t][msg.sender].verified[1] == true); allocateTokens(t+1);
            delete court[t][msg.sender];
    }
    function claimProofOfUniqueHuman() external {
            uint t = schedule();
            deductToken(t, Token.ProofOfUniqueHuman);
            proofOfUniqueHuman[t][msg.sender] = true;
            population[t]++;
    }
    function dispute() external {
            uint t = schedule()-1;
            uint id = getPair(nym[t][msg.sender].id);
            require(id != 0);
            require(!pairVerified(t, id));
            pair[t][id].disputed = true;
    }
    function reassignNym() external {
            uint t = schedule()-1;
            uint id = nym[t][msg.sender].id;
            require(pair[t][getPair(id)].disputed == true);
            delete nym[t][msg.sender];
            court[t][msg.sender].id = uint(keccak256(abi.encode(id)));
    }
    function reassignCourt() external {
            uint t = schedule()-1;
            uint id = court[t][msg.sender].id;
            require(pair[t][getCourt(t, id)].disputed == true);
            delete court[t][msg.sender].verified;
            court[t][msg.sender].id = uint(keccak256(abi.encode(0, id)));
    }
    function borderVote(uint _target) external {
            uint t = schedule();
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
}
