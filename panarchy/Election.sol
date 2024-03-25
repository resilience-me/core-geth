contract Election is Token {

    Schedule schedule = Schedule(0x0000000000000000000000000000000000000000);
    BitPeople bitPeople = BitPeople(0x0000000000000000000000000000000000000005);

    mapping (uint => address[]) election;

    mapping (uint => mapping (address => bool)) claimedSuffrageToken;

    function vote(address _validator) public {
        uint t = schedule.schedule();
        require(balanceOf[t][msg.sender] >= 1);
        balanceOf[t][msg.sender]--;
        election[t+2].push(_validator);
    }

    function allocateSuffrageToken() public {
        uint t = schedule.schedule();
        require(bitPeople.proofOfUniqueHuman(t, msg.sender));
        require(!claimedSuffrageToken[t][msg.sender]);
        balanceOf[t][msg.sender]++;
        claimedSuffrageToken[t][msg.sender] = true;
    }
}
