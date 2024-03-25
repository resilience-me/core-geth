contract Schedule {

    uint constant public genesis = 1710568800;
    uint constant public period = 4 weeks;

    function schedule() public view returns(uint) { return ((block.timestamp - genesis) / period); }
    function toSeconds(uint _t) public pure returns (uint) { return genesis + _t * period; }
    function quarter(uint _t) public view returns (uint) { return (block.timestamp-toSeconds(_t))/(period/4); }
    function hour(uint _t) public pure returns (uint) { return 1 + uint(keccak256(abi.encode(_t)))%24; }
    function pseudonymEvent(uint _t) public pure returns (uint) { return toSeconds(_t) + hour(_t)*1 hours; }
}
