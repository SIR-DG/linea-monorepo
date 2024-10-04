import { ethers } from "hardhat";
import { DeployFunction } from "hardhat-deploy/types";
import { HardhatRuntimeEnvironment } from "hardhat/types";
import { deployFromFactory } from "../scripts/hardhat/utils";
import { get1559Fees } from "../scripts/utils";
import {
  tryVerifyContractWithConstructorArgs,
  getDeployedContractAddress,
  tryStoreAddress,
  validateDeployBranchAndTags,
  getRequiredEnvVar,
} from "../common/helpers";

const func: DeployFunction = async function (hre: HardhatRuntimeEnvironment) {
  const { deployments } = hre;
  validateDeployBranchAndTags(hre.network.name);

  const contractName = "LineaSurgeXP";
  const existingContractAddress = await getDeployedContractAddress(contractName, deployments);
  const provider = ethers.provider;

  const adminAddress = getRequiredEnvVar("LINEA_SURGE_XP_ADMIN_ADDRESS");
  const minterAddress = getRequiredEnvVar("LINEA_SURGE_XP_MINTER_ADDRESS");
  const transferAddresses = getRequiredEnvVar("LINEA_SURGE_XP_TRANSFER_ADDRESSES")?.split(",");

  console.log(transferAddresses);
  if (existingContractAddress === undefined) {
    console.log(`Deploying initial version, NB: the address will be saved if env SAVE_ADDRESS=true.`);
  } else {
    console.log(`Deploying new version, NB: ${existingContractAddress} will be overwritten if env SAVE_ADDRESS=true.`);
  }
  const contract = await deployFromFactory(
    contractName,
    provider,
    adminAddress,
    minterAddress,
    transferAddresses,
    await get1559Fees(provider),
  );
  const contractAddress = await contract.getAddress();

  console.log(`${contractName} deployed at ${contractAddress}`);

  const deployTx = contract.deploymentTransaction();
  if (!deployTx) {
    throw "Contract deployment transaction receipt not found.";
  }

  await tryStoreAddress(hre.network.name, contractName, contractAddress, deployTx.hash);

  const args = [adminAddress, minterAddress, transferAddresses];

  await tryVerifyContractWithConstructorArgs(contractAddress, "contracts/token/LineaSurgeXP.sol:LineaSurgeXP", args);
};
export default func;
func.tags = ["LineaSurgeXPToken"];
