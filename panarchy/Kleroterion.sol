// An opt-in random number generator that uses potentially every single person in BitPeople (thus potentially every person in the world) to generate a random number.
// It uses a commit-reveal scheme, combined with "mutating" the committed value (thus, no one is able to know what they vote for, but they can ensure their vote is random).
// It here uses BitPeople registered addresses as the "salt" or "seed", for the "mutation". The RNG selects a number between 0 and N, and then selects the address registered 
// at that index in BitPeople in the previous period (that has been shuffled since the number was committed in this RNG. ) But it could work with any other "seed" as well as
// simply using the number between 0 and N as the "salt" to the "mutation". The RNG is intended to be used by BitPeople, that will use also use registered addresses as the "seed",
// and, it can be used by any other application on the ledger as well. It will likely be used by the validators to periodically "reset" their random number generator, making it
// even more secure.

contract Kleroterion {

    Schedule schedule = Schedule(0x0000000000000000000000000000000000000000);
    BitPeople bitpeople = BitPeople(0x0000000000000000000000000000000000000005);

    mapping (uint => bytes32) seed;

    mapping (uint => mapping (address => bytes32)) commit;
    mapping (uint => uint) votes;
    mapping (uint => mapping (uint => uint)) points;
    mapping (uint => uint) highscore;
    mapping (uint => uint) public winner;
    mapping (uint => mapping (address => bool)) claimed;
    mapping (uint => mapping (address => uint)) public balanceOf;
    mapping (uint => mapping (address => mapping (address => uint))) public allowed;

    function initSeed(uint _t) public { seed[_t] = bitpeople.registry(_t-1, winner[_t]%bitpeople.registered(_t-1)); }

    function commitHash(bytes32 _hash) public {
        uint t = schedule.schedule();
        require(schedule.quarter(t) < 2 && balanceOf[t][msg.sender] >= 1);
        balanceOf[t][msg.sender]--;
        commit[t][msg.sender] = _hash;
        votes[t]++;
    }
    function revealHash(bytes32 _preimage) public {
        uint t = schedule.schedule();
        require(schedule.quarter(t) == 2 && keccak256(abi.encode(_preimage)) == commit[t-1][msg.sender]);
        if(seed[t-1] == bytes32(0)) initSeed(_t-1);
        bytes32 mutated = keccak256(abi.encode(_preimage, seed[t-1]));
        uint id = uint(mutated)%votes[t];
        points[t][id]++;
        if (points[t][id] > highscore[t]) {
            highscore[t]++;
            winner[t] = id;
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
