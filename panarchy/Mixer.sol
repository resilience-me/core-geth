contract Mixer is Schedule {
    
    mapping (uint => mapping (address => uint)) public balanceOf;
    mapping (uint => mapping (address => mapping (address => uint))) public allowed;

    function _transfer(uint _t, address _from, address _to, uint _value) internal {
        require(balanceOf[_t][_from] >= _value);
        balanceOf[_t][_from] -= _value;
        balanceOf[_t][_to] += _value;
    }
    function transfer(address _to, uint _value) external {
        _transfer(schedule(), msg.sender, _to, _value);
    }
    function approve(address _spender, uint _value) external {
        allowed[schedule()][msg.sender][_spender] = _value;
    }
    function transferFrom(address _from, address _to, uint _value) external {
        uint t = schedule();
        require(allowed[t][_from][msg.sender] >= _value);
        _transfer(t, _from, _to, _value);
        allowed[t][_from][msg.sender] -= _value;
    }
}
