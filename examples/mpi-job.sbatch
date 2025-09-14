#!/bin/bash
#SBATCH --job-name=gromacs-md
#SBATCH --partition=aws-burst
#SBATCH --nodes=4
#SBATCH --ntasks-per-node=8
#SBATCH --cpus-per-task=1
#SBATCH --mem=16GB
#SBATCH --time=02:00:00
#SBATCH --constraint=efa-preferred
#SBATCH --output=gromacs_%j.out
#SBATCH --error=gromacs_%j.err

# Example MPI job that benefits from EFA and cluster placement groups
# This job will be automatically detected as MPI and scheduled on appropriate instances

module load gromacs/2023.1
module load openmpi/4.1.4

# Get ABSA recommendation before running
absa analyze $0 --output=absa_decision.json --job-id=$SLURM_JOB_ID

echo "Starting GROMACS MD simulation on $SLURM_NNODES nodes"
echo "Job ID: $SLURM_JOB_ID"
echo "Nodes: $SLURM_JOB_NODELIST"

# Setup environment
export OMP_NUM_THREADS=$SLURM_CPUS_PER_TASK

# Run the simulation
mpirun -np $SLURM_NTASKS \
    --mca btl ^openib \
    --mca mtl ofi \
    --mca pml cm \
    gmx_mpi mdrun -deffnm production -ntomp $OMP_NUM_THREADS

echo "Simulation completed successfully"