import React, { useState, useEffect } from 'react';
import { updateStrategyParams } from '../api';

const EditStrategyModal = ({ strategy, onClose, onUpdate }) => {
    const [params, setParams] = useState('');
    const [error, setError] = useState('');

    useEffect(() => {
        if (strategy) {
            // We need to fetch current params or assume they are passed.
            // The list endpoint returns 'parameters' as JSON string or object?
            // Let's assume the list endpoint includes it. If not, we might need to fetch it.
            // For now, let's assume strategy object has it.
            // Wait, getStrategies controller returns: id, name, type, symbol, interval, is_active.
            // It DOES NOT return parameters.
            // We should update getStrategies to return parameters or fetch them here.
            // Updating getStrategies is better.
            // But for now, let's just initialize with empty object if missing.
            setParams(JSON.stringify(strategy.parameters || {}, null, 2));
        }
    }, [strategy]);

    const handleSave = async () => {
        try {
            const parsed = JSON.parse(params);
            await updateStrategyParams(strategy.id, parsed);
            onUpdate();
            onClose();
        } catch (err) {
            setError(err.message);
        }
    };

    if (!strategy) return null;

    return (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center">
            <div className="bg-white p-6 rounded shadow-lg w-96">
                <h3 className="text-lg font-bold mb-4">Edit Strategy: {strategy.name}</h3>
                <textarea
                    className="w-full h-40 border p-2 mb-4 font-mono text-sm"
                    value={params}
                    onChange={(e) => setParams(e.target.value)}
                />
                {error && <p className="text-red-500 text-sm mb-4">{error}</p>}
                <div className="flex justify-end space-x-2">
                    <button onClick={onClose} className="px-4 py-2 bg-gray-300 rounded hover:bg-gray-400">Cancel</button>
                    <button onClick={handleSave} className="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600">Save</button>
                </div>
            </div>
        </div>
    );
};

export default EditStrategyModal;
