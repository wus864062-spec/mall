//SPDX-License-Identifier: MIT
pragma solidity ^0.8.26;

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";

/// @notice buy(num) 最低 1 USDT。仅 BSC（chainid 56）。
/// USDT：0x55d398326f99059fF775485246999027B3197955
/// 拆款地址与比例：80 / 10 / 5 / 3 / 1.5 / 0.5。
contract BuySomething is ReentrancyGuard {
    using SafeERC20 for IERC20;

    event Recharged(address indexed payer, uint256 num);

    uint256 public constant BSC_CHAIN_ID = 56;
    address public constant USDT_BSC = 0x55d398326f99059fF775485246999027B3197955;

    address public usdt;

    address public constant PORT_80 = 0xa1E54373034aaE3C00Df1b9b89B20d2df55e2CAD;
    address public constant PORT_10 = 0xE7Da6c5D90f6a88fEEa228d5C9a0611c61F7500D;
    address public constant PORT_5 = 0x623ecc54647605C220199F4d273Cf9F43fDdD5c1;
    address public constant PORT_3 = 0x907D9173ab226C698C178981c4D135f8168dD6eb;
    address public constant PORT_15 = 0x279F2B0B788b50c90ceCd74C9134D152083A87B7;
    address public constant PORT_05 = 0xd3E7fE539c291010B8948Fe19372ac9223288109;

    address[] public users;
    uint256[] public usersAmount;

    constructor(address _usdt) {
        require(block.chainid == BSC_CHAIN_ID, "bsc");
        require(_usdt == USDT_BSC, "usdt");
        usdt = _usdt;
    }

    function buy(uint256 num) external nonReentrant {
        require(block.chainid == BSC_CHAIN_ID, "bsc");
        require(1 <= num, "err num");

        uint256 amount = num * 10 ** 18;
        uint256 a80 = (amount * 800) / 1000;
        uint256 a10 = (amount * 100) / 1000;
        uint256 a5 = (amount * 50) / 1000;
        uint256 a3 = (amount * 30) / 1000;
        uint256 a15 = (amount * 15) / 1000;
        uint256 a05 = amount - a80 - a10 - a5 - a3 - a15;

        IERC20 token = IERC20(usdt);
        token.safeTransferFrom(msg.sender, PORT_80, a80);
        token.safeTransferFrom(msg.sender, PORT_10, a10);
        token.safeTransferFrom(msg.sender, PORT_5, a5);
        token.safeTransferFrom(msg.sender, PORT_3, a3);
        token.safeTransferFrom(msg.sender, PORT_15, a15);
        token.safeTransferFrom(msg.sender, PORT_05, a05);

        users.push(msg.sender);
        usersAmount.push(num);
        emit Recharged(msg.sender, num);
    }

    function getUserLength() public view returns (uint256) {
        return users.length;
    }

    function getUsers() public view returns (address[] memory) {
        return users;
    }

    function getUsersByIndex(uint256 startIndex, uint256 endIndex) public view returns (address[] memory) {
        require(endIndex >= startIndex, "bad range");
        require(endIndex < users.length, "out of range");

        address[] memory data = new address[](endIndex + 1 - startIndex);
        for (uint256 i = startIndex; i <= endIndex; i++) {
            data[i - startIndex] = users[i];
        }
        return data;
    }

    function getUsersAmountByIndex(uint256 startIndex, uint256 endIndex) public view returns (uint256[] memory) {
        require(endIndex >= startIndex, "bad range");
        require(endIndex < usersAmount.length, "out of range");

        uint256[] memory data = new uint256[](endIndex + 1 - startIndex);
        for (uint256 i = startIndex; i <= endIndex; i++) {
            data[i - startIndex] = usersAmount[i];
        }
        return data;
    }
}
