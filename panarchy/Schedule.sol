contract Schedule {

    uint constant public genesis = 1710568800;
    uint constant public period = 4 weeks;

    function schedule() public view returns(uint) { return ((block.timestamp - genesis) / period); }
}
