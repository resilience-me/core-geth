contract Election {

    Schedule schedule = Schedule(0x0000000000000000000000000000000000000000);
    BitPeople bitPeople = BitPeople(0x0000000000000000000000000000000000000005);

    mapping (uint => address[]) election;

    mapping (uint => mapping (address => bool)) suffrageToken;

    mapping (uint => mapping (address => uint)) public balanceOf;
    mapping (uint => mapping (address => mapping (address => uint))) public allowed;

    function vote(address _validator) public {
        uint t = schedule.schedule();
        require(balanceOf[t][msg.sender] >= 1);
        balanceOf[t][msg.sender]--;
        election[t+2].push(_validator);
    }

    function allocateSuffrageToken() public {
        uint t = schedule.schedule();
        require(bitPeople.proofOfUniqueHuman(t, msg.sender));
        require(!suffrageToken[t][msg.sender]);
        balanceOf[t][msg.sender]++;
        suffrageToken[t][msg.sender] = true;
    }
}
